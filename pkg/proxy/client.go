package proxy

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

var (
	clientConnected = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "comfoconnect_proxy_client_connected_bool",
			Help: "If the client is connected to the gateway.",
		},
	)
	clientMessagetoGateway = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "comfoconnect_proxy_client_message_toGateway_total",
			Help: "Number of messages sent to the gateway.",
		},
		[]string{"message_type"},
	)

	clientMessagefromGateway = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "comfoconnect_proxy_client_message_fromGateway_total",
			Help: "Number of messages received from the gateway.",
		},
		[]string{"message_type"},
	)
)

type Client struct {
	IP          string
	uuid        []byte
	toGateway   chan comfoconnect.Message
	fromGateway chan comfoconnect.Message
	quit        chan bool
	exited      chan bool
	session     *comfoconnect.Session
}

func NewClient(ip string, macAddress []byte, toGateway chan comfoconnect.Message, fromGateway chan comfoconnect.Message) *Client {
	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, macAddress...)
	//uuid = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05}

	prometheus.MustRegister(clientConnected)
	prometheus.MustRegister(clientMessagefromGateway)
	prometheus.MustRegister(clientMessagetoGateway)

	return &Client{
		IP:          ip,
		uuid:        uuid,
		toGateway:   toGateway,
		fromGateway: fromGateway,
	}
}

func (c Client) Run(ctx context.Context, wg *sync.WaitGroup) error {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "Client",
		"method": "Run",
	})

	log.Info("starting client")

	log.Debugf("starting new session with gateway %s", c.IP)
	session, err := comfoconnect.NewSession(ctx, wg, c.IP, 0, c.uuid)
	if err != nil {
		log.Errorf("failed to create a session with gateway %s: %v", c.IP, err)
		panic(err)
	}
	clientConnected.Set(1)
	c.session = session

	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down")
			c.session.Close()
			clientConnected.Set(0)
			wg.Done()
			return nil

		case message := <-c.toGateway:
			clientMessagetoGateway.WithLabelValues(message.Operation.Type.String()).Inc()

			message.Src = c.uuid // masquerade
			message.Dst = c.session.Dst // masquerade

			span := opentracing.GlobalTracer().StartSpan("proxy.Client.Run.toGateway", opentracing.ChildOf(message.Span.Context()))
			comfoconnect.SpanSetMessage(span, message)
			message.Span = span

			message.Src = c.uuid // masquerade
			message.Dst = c.session.Dst // masquerade

			log.WithField("span", span.Context().(jaeger.SpanContext).String()).Debugf("sending message to gateway: %v", message)
			err := c.session.Send(message)
			if err != nil {
				span.SetTag("err", err)
				log.Errorf("sending message to gateway failed: %v", err)
			}

			span.Finish()

		default:
			log.Debug("waiting for message from gateway")

			message, err := c.session.Receive()
			if err != nil {
				if errors.Cause(err) == io.EOF {
					log.Warn("gateway closed connection")
					clientConnected.Set(0)
					return errors.Wrap(err, "lost connection to gateway")
				}
				if opError, ok := errors.Cause(err).(*net.OpError); ok {
					if opError.Timeout() {
						break // Receive() sets a timeout, so this loop can keep running
					}
				}
				log.Errorf("got error while receiving from gateway: %v", err)
				break // restart loop
			}
			clientMessagefromGateway.WithLabelValues(message.Operation.Type.String()).Inc()
			log.Debugf("received message from gateway: %v", message)

			span := opentracing.GlobalTracer().StartSpan("proxy.Client.Run.default", opentracing.ChildOf(message.Span.Context()))
			comfoconnect.SpanSetMessage(span, message)
			message.Span = span

			switch message.Operation.Type.String() {
			case "CnTimeConfirmType":
				// ignore these, they're part of the keep-alive that the client does
			default:
				log.WithField("span", span.Context().(jaeger.SpanContext).String()).Debugf("sending message back to proxy: %v", message)
				c.fromGateway <- message
				log.WithField("span", span.Context().(jaeger.SpanContext).String()).Debugf("sent message back to proxy: %v", message)
			}
			span.Finish()
		}
	}
}
