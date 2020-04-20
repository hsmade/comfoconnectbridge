package proxy

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/proto"
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

	messageChannel := make(chan comfoconnect.Message, 10)
	go func (ctx context.Context, wg *sync.WaitGroup, messageChannel chan comfoconnect.Message) {
		log.Debug("starting socket reader")
		for {
			select {
			case <- ctx.Done():
				log.Debug("closing connection reader go-func")
				wg.Done()
				return
			default:
				err := a.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
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
			span := opentracing.GlobalTracer().StartSpan("proxy.App.HandleConnection.ReceivedMessage", opentracing.ChildOf(message.Span.Context()))
			comfoconnect.SpanSetMessage(span, message)
			log.WithField("span",span.Context().(jaeger.SpanContext).String()).Debugf("got a message from app(%s): %v", a.conn.RemoteAddr(), message)
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
		_, err := a.conn.Write(message.CreateResponse(span, proto.GatewayOperation_OK))
		if err != nil {
			span.SetTag("err", err)
			log.Warnf("failed to write response for RegisterAppRequestType: %v", err)
		}
	case "StartSessionRequestType":
		log.Debug("responding to StartSessionRequestType")
		_, err := a.conn.Write(message.CreateResponse(span, proto.GatewayOperation_OK))
		if err != nil {
			span.SetTag("err", err)
			log.Warnf("failed to write response for StartSessionRequestType: %v", err)
		}

		i := uint32(1)
		mode := proto.CnNodeNotification_NODE_NORMAL
		notification := proto.CnNodeNotification{
			NodeId:    &i,
			ProductId: &i,
			ZoneId:    &i,
			Mode:      &mode,
		}
		_, err = a.conn.Write(message.CreateCustomResponse(span, proto.GatewayOperation_CnNodeNotificationType, &notification))
		if err != nil {
			span.SetTag("err", err)
			log.Warnf("failed to write CnNodeNotification-1: %v", err)
		}

		i48 := uint32(48)
		i5 := uint32(5)
		i255 := uint32(255)
		mode = proto.CnNodeNotification_NODE_NORMAL
		notification = proto.CnNodeNotification{
			NodeId:    &i48,
			ProductId: &i5,
			ZoneId:    &i255,
			Mode:      &mode,
		}
		_, err = a.conn.Write(message.CreateCustomResponse(span, proto.GatewayOperation_CnNodeNotificationType, &notification))
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
	span := opentracing.GlobalTracer().StartSpan("proxy.App.Write")
	defer span.Finish()
	length, err := a.conn.Write(message.Encode())
	span.SetTag("length", length)
	return err
}
