package proxy

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"

	"github.com/hsmade/comfoconnectbridge/pb"
	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

var (
	clientConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "comfoconnect_proxy_listener_connections",
			Help: "Number of connections to the listener.",
		},
	)
	messageSentCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "comfoconnect_proxy_listener_message_sent_total",
			Help: "Number of messages sent by the listener.",
		},
		[]string{"message_type"},
	)
	messageReceiverCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "comfoconnect_proxy_listener_message_receiver_total",
			Help: "Number of messages received by the listener go func.",
		},
		[]string{"message_type"},
	)
	messageReceivedCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "comfoconnect_proxy_listener_message_received_total",
			Help: "Number of messages received by the listener main loop.",
		},
		[]string{"message_type"},
	)
)

type Listener struct {
	listener  *net.TCPListener

	apps      map[string]*App
	toGateway chan comfoconnect.Message
}

func NewListener(toGateway chan comfoconnect.Message) *Listener {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "NewListener",
	})

	addr, err := net.ResolveTCPAddr("tcp4", ":56747")
	if err != nil {
		log.Fatalf("failed to resolve address: %v", err)
	}

	log.Infof("Listening on %s", addr.String())
	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}

	prometheus.MustRegister(clientConnections)
	prometheus.MustRegister(messageSentCount)
	prometheus.MustRegister(messageReceiverCount)
	prometheus.MustRegister(messageReceivedCount)
	return &Listener{

		listener:  listener,
		toGateway: toGateway,
		apps:      make(map[string]*App),
	}
}

func (l *Listener) Run(ctx context.Context, wg *sync.WaitGroup) {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "listener",
		"method": "Run",
	})

	log.Debug("starting")
	var handlers sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down")
			_ = l.listener.Close()
			handlers.Wait()
			wg.Done()
			return

		default:
			err := l.listener.SetDeadline(time.Now().Add(time.Second * 1))
			if err != nil {
				log.Errorf("failed to set read deadline: %v", err)
				continue
			}

			conn, err := l.listener.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				log.Errorf("failed to accept connection: %v", err)
				continue
			}
			clientConnections.Inc()
			log.Infof("got a new connection from: %s", conn.RemoteAddr().String())
			handlers.Add(1)
			go func() {
				for {
					app := App{conn: conn}
					l.apps[conn.RemoteAddr().String()] = &app
					log.Debug("starting handler")
					err := app.HandleConnection(ctx, wg, l.toGateway)
					if err != nil {
						log.Errorf("failed to handle connection: %v", err)
						break
					}
				}
				handlers.Done()
				clientConnections.Dec()
			}()
		}
	}
}

type App struct {
	uuid []byte
	conn net.Conn
}

func (a *App) HandleConnection(ctx context.Context, wg *sync.WaitGroup, gateway chan comfoconnect.Message) error {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "listener",
		"method": "HandleConnection",
	})

	log.Infof("handling connection from: %s", a.conn.RemoteAddr().String())

	messageChannel := make(chan comfoconnect.Message, 500)
	//prometheus.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
	//	Name: "comfoconnect_proxy_listener_messageChannel_queue",
	//	Help: "The current number of items on messageChannel queue.",
	//}, func() float64 {
	//	return float64(len(messageChannel))
	//}))

	go func (ctx context.Context, wg *sync.WaitGroup, messageChannel chan comfoconnect.Message) {
		log.Debug("starting socket reader")
		for {
			select {
			case <- ctx.Done():
				log.Debug("closing connection reader go-func")
				wg.Done()
				return
			default:
				err := a.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 300))
				if err != nil {
					log.Warnf("failed to set readDeadline: %v", err)
				}

				message, err := comfoconnect.GetMessageFromSocket(a.conn)
				if err != nil {
					if errors.Cause(err) == io.EOF {
						return
					}
					// FIXME: log error, ignore timeout
					continue
				}
				messageReceiverCount.WithLabelValues(message.Operation.Type.String()).Inc()
				messageChannel <- message
			}
		}
	}(ctx, wg, messageChannel)

	log.Debug("starting main loop")
	for {
		select {
		case <- ctx.Done():
			log.Debug("closing main loop")
			wg.Done()
			return nil
		case message := <- messageChannel:
			messageReceivedCount.WithLabelValues(message.Operation.Type.String()).Inc()
			span := opentracing.GlobalTracer().StartSpan("proxy.App.HandleConnection.ReceivedMessage", opentracing.ChildOf(message.Span.Context()))
			comfoconnect.SpanSetMessage(span, message)
			log.WithField("span",span.Context().(jaeger.SpanContext).String()).Debugf("got a message from app(%s): %v", a.conn.RemoteAddr(), message)
			a.uuid = message.Src
			a.handleMessage(message, gateway)
			span.Finish()
		}
	}
}

func (a *App) handleMessage(message comfoconnect.Message, gateway chan comfoconnect.Message) {
	span := opentracing.GlobalTracer().StartSpan("proxy.App.HandleConnection.handleMessage", opentracing.ChildOf(message.Span.Context()))
	comfoconnect.SpanSetMessage(span, message)

	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "listener",
		"method": "handleMessage",
		"span": span.Context().(jaeger.SpanContext).String(),
	})

	switch message.Operation.Type.String() {
	case "RegisterAppRequestType":
		log.Debug("responding to RegisterAppRequestType")
		a.uuid = message.Src
		_, err := a.conn.Write(message.CreateResponse(span, pb.GatewayOperation_OK))
		if err != nil {
			span.SetTag("err", err)
			log.Warnf("failed to write response for RegisterAppRequestType: %v", err)
		}
	case "StartSessionRequestType":
		log.Debug("responding to StartSessionRequestType")
		_, err := a.conn.Write(message.CreateResponse(span, pb.GatewayOperation_OK))
		if err != nil {
			span.SetTag("err", err)
			log.Warnf("failed to write response for StartSessionRequestType: %v", err)
		}

		i := uint32(1)
		mode := pb.CnNodeNotification_NODE_NORMAL
		notification := pb.CnNodeNotification{
			NodeId:    &i,
			ProductId: &i,
			ZoneId:    &i,
			Mode:      &mode,
		}
		_, err = a.conn.Write(message.CreateCustomResponse(span, pb.GatewayOperation_CnNodeNotificationType, &notification))
		if err != nil {
			span.SetTag("err", err)
			log.Warnf("failed to write CnNodeNotification-1: %v", err)
		}

		i48 := uint32(48)
		i5 := uint32(5)
		i255 := uint32(255)
		mode = pb.CnNodeNotification_NODE_NORMAL
		notification = pb.CnNodeNotification{
			NodeId:    &i48,
			ProductId: &i5,
			ZoneId:    &i255,
			Mode:      &mode,
		}
		_, err = a.conn.Write(message.CreateCustomResponse(span, pb.GatewayOperation_CnNodeNotificationType, &notification))
		if err != nil {
			span.SetTag("err", err)
			log.Warnf("failed to write CnNodeNotification-2: %v", err)
		}

	default:
		log.Debugf("forwarding message to gateway: %v", message)
		message.Span = span
		gateway <- message
	}
	span.Finish()
}

func (a *App) Write(message comfoconnect.Message) error {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "App",
		"method": "Write",
	})
	messageSentCount.WithLabelValues(message.Operation.Type.String()).Inc()
	span := opentracing.GlobalTracer().StartSpan("proxy.App.Write")
	comfoconnect.SpanSetMessage(span, message)
	defer span.Finish()
	e := message.Encode()
	length, err := a.conn.Write(e)
	log.Infof("Wrote %d bytes to app. err:%v bytes:%x message:%v", length, err, message.Encode(), message)
	span.SetTag("length", length)
	return err
}
