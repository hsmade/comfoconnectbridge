package proxy

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Proxy struct {
	client    *Client
	uuid      []byte
	listener  *Listener
	toGateway chan *comfoconnect.Message
	quit      chan bool
	exited    chan bool
}

func NewProxy(ctx context.Context, comfoConnectIP string, myMacAddress []byte) *Proxy {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "NewProxy",
	})

	//tracer := opentracing.GlobalTracer()
	//span := tracer.StartSpan("proxy.NewProxy")
	//ctx = opentracing.ContextWithSpan(ctx, span)
	//defer span.Finish()

	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, myMacAddress...)

	toGateway := make(chan *comfoconnect.Message)
	log.Info("creating new listener")
	l := NewListener(ctx, toGateway)
	log.Info("creating new cient")
	c := NewClient(ctx, comfoConnectIP, myMacAddress, toGateway)

	p := Proxy{
		client:    c,
		listener:  l,
		uuid:      uuid,
		toGateway: toGateway,
	}

	return &p
}

func (p Proxy) Run(ctx context.Context) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Proxy",
		"method": "Run",
	})

	//span, _ := opentracing.StartSpanFromContext(ctx, "proxy.Proxy.Run")
	//defer span.Finish()

	log.Info("starting new cient")
	go p.client.Run(ctx)
	log.Info("starting new listener")
	go p.listener.Run(ctx)

	for {
		select {
		case <-p.quit:
			log.Info("Shutting down proxy server")
			p.client.Stop(ctx)
			p.listener.Stop(ctx)
			close(p.exited)
			return
		case message := <-p.toGateway:
			log.Debugf("received a message: %s", *message)
			generateMetrics(message.Ctx, message)
			message.Src = p.uuid
			log.Debugf("forwarding message to gateway: %s", *message)
			p.client.toGateway <- message
		case message := <-p.client.fromGateway:
			log.Debugf("received a message from gateway: %s", *message)
			generateMetrics(message.Ctx, message)
			for _, app := range p.listener.apps {
				log.Debugf("copying message from gateway to app(%s/%x):%v", app.conn.RemoteAddr().String(), app.uuid)
				message.Dst = app.uuid
				err := app.Write(ctx, message)
				if err != nil {
					log.Errorf("error while copying message from gateway to app(%s/%x):%v", app.conn.RemoteAddr().String(), app.uuid, err)
				}
			}
		}
	}
}

func (p *Proxy) Stop(ctx context.Context) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Proxy",
		"method": "Stop",
	})

	//span, _ := opentracing.StartSpanFromContext(ctx, "proxy.Proxy.Stop")
	//defer span.Finish()

	log.Debug("Stopping")
	close(p.quit)
	<-p.exited
	log.Info("Stopped")
}

func generateMetrics(ctx context.Context, message *comfoconnect.Message) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Proxy",
		"method": "generateMetrics",
	})

	span, _ := opentracing.StartSpanFromContext(ctx, "proxy.generateMetrics")
	defer span.Finish()

	log.Debugf("called for %s", *message)
}
