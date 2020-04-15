package proxy

import (
	"context"
	"io"
	"net"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Client struct {
	IP          string
	uuid        []byte
	toGateway   chan *comfoconnect.Message
	fromGateway chan *comfoconnect.Message
	quit        chan bool
	exited      chan bool
	session     *comfoconnect.Session
}

func NewClient(ctx context.Context, ip string, macAddress []byte, toGateway chan *comfoconnect.Message) *Client {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "NewClient",
	})

	span, _ := opentracing.StartSpanFromContext(ctx, "proxy.NewClient")
	defer span.Finish()

	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, macAddress...)

	log.Debugf("starting new session with gateway %s", ip)
	session, err := comfoconnect.CreateSession(ctx, ip, 0, uuid)
	if err != nil {
		log.Errorf("failed to create a session with gateway %s: %v", ip, err)
		panic(err)
	}

	return &Client{
		IP:          ip,
		uuid:        uuid,
		toGateway:   toGateway,
		fromGateway: make(chan *comfoconnect.Message),
		session:     session,
	}
}

func (c Client) Run(ctx context.Context) error {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "Client",
		"method": "Run",
	})

	tracer := opentracing.GlobalTracer()
	span := tracer.StartSpan("gateway client")
	ctx = opentracing.ContextWithSpan(ctx, span)
	defer span.Finish()

	log.Info("starting")
	for {
		select {
		case <-c.quit:
			log.Info("Shutting down")
			c.session.Close(ctx)
			close(c.exited)
			return nil

		case m := <-c.toGateway:
			log.Debugf("sending message to gateway: %v", m)
			err := c.session.Send(m.Ctx, m)
			if err != nil {
				log.Errorf("sending message to gateway failed: %v", err)
			}
		default:
			log.Debug("waiting for message from gateway")
			m, err := c.session.Receive(ctx)
			if err != nil {
				if errors.Cause(err) == io.EOF {
					log.Warn("gateway closed connection")
					return errors.Wrap(err, "lost connection to gateway")
				}
				if errors.Cause(err).(*net.OpError).Timeout()  {
					break // Receive() sets a timeout, so this loop can keep running
				}
				log.Errorf("got error while receiving from gateway: %v", err)
				break // restart loop
			}
			log.Debugf("received message from gateway: %v", m)
			c.fromGateway <- m
		}
	}
}

func (c *Client) Stop(ctx context.Context) {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "Client",
		"method": "Stop",
	})

	span, _ := opentracing.StartSpanFromContext(ctx, "proxy.Client.Stop")
	defer span.Finish()

	log.Debug("Stopping")
	close(c.quit)
	<-c.exited
	log.Info("Stopped")
}
