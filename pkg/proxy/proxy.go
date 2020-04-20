package proxy

import (
	"context"
	"sync"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Proxy struct {
	client      *Client
	uuid        []byte
	listener    *Listener
	toGateway   chan comfoconnect.Message
	fromGateway chan comfoconnect.Message
	quit        chan bool
	exited      chan bool
}

var (
	proxyMessagetoGateway = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "comfoconnect_proxy_proxy_message_toGateway_total",
			Help: "Number of messages sent to the gateway.",
		},
		[]string{"message_type"},
	)
	proxyMessagefromGateway = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "comfoconnect_proxy_proxy_message_fromGateway_total",
			Help: "Number of messages received from the gateway.",
		},
		[]string{"message_type"},
	)
	metricsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comfoconnect_pdo_value",
			Help: "Value for the different PDOs, as they're seen by the proxy",
		},
		[]string{"ID", "description"},
	)
)

func NewProxy(gatewayIP string, myMacAddress []byte) *Proxy {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "NewProxy",
	})

	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, myMacAddress...)

	prometheus.MustRegister(proxyMessagefromGateway)
	prometheus.MustRegister(proxyMessagetoGateway)
	prometheus.MustRegister(metricsGauge)

	toGateway := make(chan comfoconnect.Message, 50)
	prometheus.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "comfoconnect_proxy_proxy_toGateway_queue_length",
		Help: "The current number of items on toGateway queue.",
	}, func() float64 {
		return float64(len(toGateway))
	}))

	fromGateway := make(chan comfoconnect.Message, 50)
	prometheus.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "comfoconnect_proxy_proxy_fromGateway_queue_length",
		Help: "The current number of items on fromGateway queue.",
	}, func() float64 {
		return float64(len(fromGateway))
	}))

	log.Info("creating new listener")
	l := NewListener(toGateway)
	log.Info("creating new client")
	c := NewClient(gatewayIP, myMacAddress, toGateway, fromGateway)

	p := Proxy{
		client:      c,
		listener:    l,
		uuid:        uuid,
		toGateway:   toGateway,
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
	go func(){
		err := p.client.Run(ctx, wg)
		if err != nil {

			log.Errorf("client exited: %v", err)
		}
	}()

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
			proxyMessagetoGateway.WithLabelValues(message.Operation.Type.String()).Inc()
			log.Debugf("received a message for the gateway: %v", message)
			span := opentracing.GlobalTracer().StartSpan("proxy.Proxy.Run.ReceivedForGateway", opentracing.ChildOf(message.Span.Context()))
			comfoconnect.SpanSetMessage(span, message)
			message.Span = span

			generateMetrics(message)
			message.Src = p.uuid // masquerade

			log.WithField("span", span.Context().(jaeger.SpanContext).String()).Debugf("forwarding message to gateway: %v", message)
			p.client.toGateway <- message

			span.Finish()

		case message := <-p.fromGateway:
			proxyMessagefromGateway.WithLabelValues(message.Operation.Type.String()).Inc()
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
		"span":   span.Context().(jaeger.SpanContext).String(),
	})

	switch message.Operation.Type.String() {
	case "CnRpdoNotificationType":
		conv := message.DecodePDO()
		metricsGauge.WithLabelValues(conv.GetDescription()).Set(conv.Tofloat64())
	case "CnAlarmNotificationType":
		log.Warnf("Got alarm notification: %v", message)
	}
	log.Debugf("called for %v", message)
}
