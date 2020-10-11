package dumbproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/hsmade/comfoconnectbridge/pb"
	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/helpers"
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
	prometheus.MustRegister(metricsGauge)
	helpers.StackLogger().Info("starting proxy")

	addr, err := net.ResolveTCPAddr("tcp4", ":56747")
	if err != nil {
		helpers.PanicOnError(errors.Wrap(err, "failed to resolve address"))
	}

	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		helpers.PanicOnError(errors.Wrap(err, "failed to create listener"))
	}

	for {
		select {
		case <-ctx.Done():
			_ = listener.Close()
			wg.Done()
		default:
			err := listener.SetDeadline(time.Now().Add(time.Millisecond * 100))
			if err != nil {
				helpers.StackLogger().Errorf("failed to set read deadline: %v", err)
				continue
			}

			listenerConnection, err := listener.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				helpers.StackLogger().Errorf("failed to accept connection: %v", err)
				continue
			}

			gatewayConnection, err := net.Dial("tcp", fmt.Sprintf("%s:56747", d.GatewayIP))
			if err != nil {
				helpers.PanicOnError(errors.Wrap(err, "connecting to gw")) // no use to linger around if we can't connect
			}
			helpers.StackLogger().Debugf("connected to %s", gatewayConnection.RemoteAddr())

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
	for {
		err := conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		if err != nil {
			helpers.StackLogger().Warnf("failed to set readDeadline: %v", err)
		}

		message, err := comfoconnect.NewMessageFromSocket(conn)
		if err == nil {
			helpers.StackLogger().Infof("received %v from %s", message, conn.RemoteAddr().String())
			if message.Operation.Type != nil {
				generateMetrics(*message)
				channel <- message.Encode()
			}
		} else {
			if errors.Cause(err) == io.EOF {
				helpers.PanicOnError(errors.New("client left"))
			}
			helpers.StackLogger().Debugf("receive err: %v", err)
		}
	}
}

func (d DumbProxy) proxySend(conn net.Conn, channel chan []byte) {
	for {
		b := <-channel
		length, err := conn.Write(b)
		if err == nil {
			helpers.StackLogger().Infof("sent %d bytes: %x to %s", length, b[:length], conn.RemoteAddr().String())
		} else {
			helpers.StackLogger().Debugf("send err: %v", err)
		}
	}
}

func generateMetrics(message comfoconnect.Message) {
	switch message.Operation.Type.String() {
	case "CnRpdoRequestType":
		b := message.OperationType.(*pb.CnRpdoRequest)
		helpers.StackLogger().Infof("CnRpdoRequestType: ppid:%d type:%d zone:%d", *b.Pdid, *b.Type, *b.Zone)
	case "CnRpdoNotificationType":
		conv := message.DecodePDO()
		helpers.StackLogger().Infof("Got RPDO: %s %v with value %f", reflect.TypeOf(conv), conv, conv.Tofloat64())
		metricsGauge.WithLabelValues(conv.GetID(), conv.GetDescription()).Set(conv.Tofloat64())
	case "CnAlarmNotificationType":
		helpers.StackLogger().Warnf("Got alarm notification: %v", message)
	}
	helpers.StackLogger().Debugf("called for %v", message)
}
