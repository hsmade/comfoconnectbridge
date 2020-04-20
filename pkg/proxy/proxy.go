package proxy

import (
	"context"
	"sync"

	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Proxy struct {
	client    *Client
	uuid      []byte
	listener  *Listener
	toGateway chan comfoconnect.Message
	fromGateway chan comfoconnect.Message
	quit      chan bool
	exited    chan bool
}

func NewProxy(comfoConnectIP string, myMacAddress []byte) *Proxy {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "NewProxy",
	})

	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, myMacAddress...)

	toGateway := make(chan comfoconnect.Message,10)
	fromGateway := make(chan comfoconnect.Message, 10)

	log.Info("creating new listener")
	l := NewListener(toGateway)
	log.Info("creating new cient")
	c := NewClient(comfoConnectIP, myMacAddress, toGateway, fromGateway)

	p := Proxy{
		client:    c,
		listener:  l,
		uuid:      uuid,
		toGateway: toGateway,
		fromGateway: fromGateway,
	}

	return &p
}

func (p Proxy) Run(ctx context.Context, wg *sync.WaitGroup) {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "Proxy",
		"method": "Run",
	})

	log.Info("starting new client")
	wg.Add(1)
	go p.client.Run(ctx, wg)

	log.Info("starting new listener")
	wg.Add(1)
	go p.listener.Run(ctx, wg)

	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down proxy server")
			wg.Wait()
			return

		case message := <-p.toGateway:
			log.Debugf("received a message for the gateway: %v", message)
			span := opentracing.GlobalTracer().StartSpan("proxy.Proxy.Run.ReceivedForGateway", opentracing.ChildOf(message.Span.Context()))
			comfoconnect.SpanSetMessage(span, message)
			message.Span = span

			generateMetrics(message)
			message.Src = p.uuid // masquerade

			log.WithField("span", span.Context().(jaeger.SpanContext).String(),
		).Debugf("forwarding message to gateway: %v", message)
			p.client.toGateway <- message

			span.Finish()

		case message := <-p.fromGateway:
			span := opentracing.GlobalTracer().StartSpan("proxy.Proxy.Run.ReceivedFromGateway", opentracing.ChildOf(message.Span.Context()))
			comfoconnect.SpanSetMessage(span, message)
			message.Span = span

			log.WithField("span", span.Context().(jaeger.SpanContext).String()).Debugf("received a message from gateway: %v", message)
			generateMetrics(message)

			log.WithField("span", span.Context().(jaeger.SpanContext).String()).Debugf("going to copy to %d apps", len(p.listener.apps))
			for _, app := range p.listener.apps {
				message.Dst = app.uuid // masquerade
				log.Debugf("copying message from gateway to app(%s/%x):%v", app.conn.RemoteAddr().String(), app.uuid, message)
				err := app.Write(message)
				if err != nil {
					log.Errorf("error while copying message from gateway to app(%s/%x):%v", app.conn.RemoteAddr().String(), app.uuid, err)
				}
			}

			span.Finish()
		}
	}
}

func generateMetrics(message comfoconnect.Message) {
	span := opentracing.GlobalTracer().StartSpan("proxy.generateMetrics", opentracing.ChildOf(message.Span.Context()))
	comfoconnect.SpanSetMessage(span, message)
	defer span.Finish()

	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "generateMetrics",
		"span": span.Context().(jaeger.SpanContext).String(),
	})

	log.Debugf("called for %v", message)
}
