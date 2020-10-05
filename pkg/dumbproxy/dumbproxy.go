package dumbproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pb"
)

var (
	metricsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comfoconnect_pdo_value",
			Help: "Value for the different PDOs, as they're seen by the proxy",
		},
		[]string{"ID", "description"},
	)
)

type DumbProxy struct {
	GatewayIP string
}

func (d DumbProxy) Run(ctx context.Context, wg *sync.WaitGroup) {
	log := logrus.WithFields(logrus.Fields{
		"module": "dumbproxy",
		"object": "DumbProxy",
		"method": "Run",
	})
	prometheus.MustRegister(metricsGauge)

	log.Info("starting proxy")

	addr, err := net.ResolveTCPAddr("tcp4", ":56747")
	if err != nil {
		log.Fatalf("failed to resolve address: %v", err)
	}

	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			_ = listener.Close()
			wg.Done()
		default:
			err := listener.SetDeadline(time.Now().Add(time.Millisecond * 100))
			if err != nil {
				log.Errorf("failed to set read deadline: %v", err)
				continue
			}

			listenerConnection, err := listener.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				log.Errorf("failed to accept connection: %v", err)
				continue
			}

			gatewayConnection, err := net.Dial("tcp", fmt.Sprintf("%s:56747", d.GatewayIP))
			if err != nil {
				log.Errorf("connect to gw: %v", err)
				return
			}
			log.Debugf("connected to %s", gatewayConnection.RemoteAddr())

			listenerChan := make(chan []byte, 100)
			gatewayChannel := make(chan []byte, 100)

			go d.proxyReceiveMessage(listenerConnection, listenerChan) // app - > chan
			go d.proxySend(listenerConnection, gatewayChannel)         // chan -> app

			go d.proxyReceiveMessage(gatewayConnection, gatewayChannel) // gw   -> chan
			go d.proxySend(gatewayConnection, listenerChan)             // chan -> gw
		}
	}
}

func (d DumbProxy) proxyReceiveMessage(conn net.Conn, channel chan []byte) {
	log := logrus.WithFields(logrus.Fields{
		"module": "dumbproxy",
		"object": "DumbProxy",
		"method": "proxyReceive",
	})
	for {
		err := conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		if err != nil {
			log.Warnf("failed to set readDeadline: %v", err)
		}

		message, err := comfoconnect.GetMessageFromSocket(conn)
		if err == nil {
			log.Infof("received %v from %s", message, conn.RemoteAddr().String())
			if message.Operation.Type != nil {
				generateMetrics(message)
				channel <- message.Encode()
			}
		} else {
			if errors.Cause(err) == io.EOF {
				log.Error("client left")
				os.Exit(0)
			}
			log.Debugf("receive err: %v", err)
		}
	}
}

func (d DumbProxy) proxySend(conn net.Conn, channel chan []byte) {
	log := logrus.WithFields(logrus.Fields{
		"module": "dumbproxy",
		"object": "DumbProxy",
		"method": "proxySend",
	})
	for {
		b := <-channel
		length, err := conn.Write(b)
		if err == nil {
			log.Infof("sent %d bytes: %x to %s", length, b[:length], conn.RemoteAddr().String())
		} else {
			log.Debugf("send err: %v", err)
		}
	}
}

func generateMetrics(message comfoconnect.Message) {

	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "generateMetrics",
	})

	switch message.Operation.Type.String() {
	case "CnRpdoRequestType":
		b := message.OperationType.(*pb.CnRpdoRequest)
		log.Infof("CnRpdoRequestType: ppid:%d type:%d zone:%d", *b.Pdid, *b.Type, *b.Zone)
	case "CnRpdoNotificationType":
		conv := message.DecodePDO()
		log.Infof("Got RPDO: %s %v with value %f", reflect.TypeOf(conv), conv, conv.Tofloat64())
		metricsGauge.WithLabelValues(conv.GetID(), conv.GetDescription()).Set(conv.Tofloat64())
	case "CnAlarmNotificationType":
		log.Warnf("Got alarm notification: %v", message)
	}
	log.Debugf("called for %v", message)
}
