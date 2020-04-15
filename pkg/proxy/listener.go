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

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/proto"
)

type Listener struct {
	listener  *net.TCPListener
	quit      chan bool
	exited    chan bool
	apps      map[string]*App
	toGateway chan *comfoconnect.Message
}

func NewListener(ctx context.Context, toGateway chan *comfoconnect.Message) *Listener {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "NewListener",
	})

	span, _ := opentracing.StartSpanFromContext(ctx, "proxy.NewListener")
	defer span.Finish()

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
		quit:      make(chan bool),
		exited:    make(chan bool),
		listener:  listener,
		toGateway: toGateway,
		apps:      make(map[string]*App),
	}
}

func (l *Listener) Run(ctx context.Context) {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "listener",
		"method": "Run",
	})

	span, _ := opentracing.StartSpanFromContext(ctx, "proxy.Listener.Run")
	defer span.Finish()

	log.Debug("starting")
	var handlers sync.WaitGroup
	for {
		select {
		case <-l.quit:
			log.Info("shutting down")
			l.listener.Close()
			handlers.Wait()
			close(l.exited)
			return

		default:
			err := l.listener.SetDeadline(time.Now().Add(time.Second * 1))
			if err != nil {
				log.Errorf("failed to set read deadline: %v", err)
				continue
			}

			// log.Debug("waiting for new connections")
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
					tracer := opentracing.GlobalTracer()
					span := tracer.StartSpan("listener connection handler")
					span.SetTag("remote", conn.RemoteAddr().String())
					ctx = opentracing.ContextWithSpan(ctx, span)
					log.Debugf("Listener SPAN: %v", span)

					app := App{conn: conn}
					l.apps[conn.RemoteAddr().String()] = &app
					log.Debug("starting handler")
					err := app.HandleConnection(ctx, l.toGateway)
					if err != nil {
						log.Errorf("failed to handle connection: %v", err)
						span.Finish()
						break
					}
				}
				handlers.Done()
			}()
		}
	}
}

func (l *Listener) Stop(ctx context.Context) {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "listener",
		"method": "Stop",
	})

	span, _ := opentracing.StartSpanFromContext(ctx, "proxy.Listener.Stop")
	defer span.Finish()

	log.Debug("stopping")
	close(l.quit)
	<-l.exited
	log.Info("stopped")
}

type App struct {
	uuid []byte
	conn net.Conn
}

func (a *App) HandleConnection(ctx context.Context, gateway chan *comfoconnect.Message) error {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "listener",
		"method": "HandleConnection",
	})
	span, _ := opentracing.StartSpanFromContext(ctx, "HandleConnection")
	defer span.Finish()

	for {
		//read message
		a.conn.SetReadDeadline(time.Now().Add(time.Second * 1))
		message, err := comfoconnect.GetMessageFromSocket(ctx, a.conn)
		if err != nil {
			if errors.Cause(err) == io.EOF {
				return err
			}
			// FIXME: log error, ignore timeout
			continue
		}
		log.Debugf("got a message from app(%s): %s", a.conn.RemoteAddr(), *message)

		switch message.Operation.Type.String() {
		case "RegisterAppRequestType":
			log.Debug("responding to RegisterAppRequestType")
			a.uuid = message.Src
			a.conn.Write(message.CreateResponse(ctx, proto.GatewayOperation_OK))
		case "StartSessionRequestType":
			log.Debug("responding to StartSessionRequestType")
			a.conn.Write(message.CreateResponse(ctx, proto.GatewayOperation_OK))

			i := uint32(1)
			mode := proto.CnNodeNotification_NODE_NORMAL
			notification := proto.CnNodeNotification{
				NodeId:    &i,
				ProductId: &i,
				ZoneId:    &i,
				Mode:      &mode,
			}
			a.conn.Write(message.CreateCustomResponse(ctx, proto.GatewayOperation_CnNodeNotificationType, &notification))

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
			a.conn.Write(message.CreateCustomResponse(ctx, proto.GatewayOperation_CnNodeNotificationType, &notification))
		default:
			log.Debugf("forwarding message to gateway: %s", *message)
			gateway <- message
		}
	}
}

func (a *App) Write(ctx context.Context, message *comfoconnect.Message) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "proxy.App.Write")
	defer span.Finish()

	_, err := a.conn.Write(message.Encode(ctx))
	return err
}
