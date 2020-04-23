package dumbproxy

import (
	"context"
	"encoding/binary"
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
	"github.com/hsmade/comfoconnectbridge/proto"
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
			listener.Close()
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

			//wg.Add(1)
			//go d.handleClient(ctx, wg, listenerConnection, gatewayConnection)
			listenerChan := make(chan []byte, 100)
			gatewayChannel := make(chan []byte, 100)

			go d.proxyReceiveMessage(listenerConnection, listenerChan)
			go d.proxyReceiveMessage(gatewayConnection, gatewayChannel)
			go d.proxySend(gatewayConnection, listenerChan)
			go d.proxySend(listenerConnection, gatewayChannel)
		}
	}
}

func (d DumbProxy) proxyReceiveBytes(conn net.Conn, channel chan []byte) {
	log := logrus.WithFields(logrus.Fields{
		"module": "dumbproxy",
		"object": "DumbProxy",
		"method": "proxyReceive",
	})
	for {
		err := conn.SetReadDeadline(time.Now().Add(time.Millisecond * 10))
		if err != nil {
			log.Warnf("failed to set readDeadline: %v", err)
		}

		b := make([]byte, 4096)
		length, err := conn.Read(b)
		if err == nil {
			log.Infof("received %d bytes: %x from %s", length, b[:length], conn.RemoteAddr().String())
				channel <- b[:length]
		//} else {
		//	log.Debugf("receive err: %v", err)
		}
	}
}

func (d DumbProxy) proxyReceiveBinaryMessage(conn net.Conn, channel chan []byte) {
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

		message := ReadMessage(conn)
		if message != nil && len(message) > 0 {
			log.Infof("received %d bytes: %x from %s", len(message), message, conn.RemoteAddr().String())
			channel <- message

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
		b := <- channel
		length, err := conn.Write(b)
		if err == nil {
			log.Infof("sent %d bytes: %x to %s", length, b[:length], conn.RemoteAddr().String())
		} else {
			log.Debugf("send err: %v", err)
		}
	}
}


func ReadMessage(conn net.Conn) []byte {
	log := logrus.WithFields(logrus.Fields{
		"module": "dumbproxy",
		"object": "DumbProxy",
		"method": "ReadMessage",
	})
	var completeMessage []byte

	lengthBytes, _ := ReadBytes(conn, 4)
	if len(lengthBytes) < 4 {
		return nil
	}
	completeMessage = append(completeMessage, lengthBytes...)
	length := binary.BigEndian.Uint32(lengthBytes)

	src, err := ReadBytes(conn, 16)
	if err != nil {
		log.Errorf("src: %v", err)
		return nil
	}
	completeMessage = append(completeMessage, src...)

	dst, err := ReadBytes(conn, 16)
	if err != nil {
		log.Errorf("dst: %v", err)
		return nil
	}
	completeMessage = append(completeMessage, dst...)

	operationLengthBytes, err := ReadBytes(conn, 2)
	if err != nil {
		log.Errorf("operationLengthBytes: %v", err)
		return nil
	}
	completeMessage = append(completeMessage, operationLengthBytes...)
	operationLength := binary.BigEndian.Uint16(operationLengthBytes)
	//operationLength = 4 // FIXME: sign error above?

	operationBytes, err := ReadBytes(conn, int(operationLength))
	if err != nil {
		log.Errorf("operationBytes: %v", err)
		return nil
	}
	completeMessage = append(completeMessage, operationBytes...)

	var operationTypeBytes []byte
	operationTypeLength := (length - 34) - uint32(operationLength)
	if operationTypeLength > 0 {
		operationTypeBytes, err = ReadBytes(conn, int(operationTypeLength))
		if err != nil {
			log.Errorf("operationTypeBytes: %v", err)
			return nil
		}
		completeMessage = append(completeMessage, operationTypeBytes...)
	}

	log.Infof("%x", completeMessage)
	operation := proto.GatewayOperation{} // FIXME: parse instead of assume
	err = operation.XXX_Unmarshal(operationBytes)
	if err != nil {
		err := errors.Wrap(err, fmt.Sprintf("failed to unmarshal operation with bytes(%d): %x", operationLength, operationBytes))
		log.Error(err)
		//return nil
	}

	operationType := comfoconnect.GetStructForType(operation.Type.String())
	err = operationType.XXX_Unmarshal(operationTypeBytes)
	if err != nil {
		err := errors.Wrap(err, "failed to unmarshal operation type") // FIXME
		log.Error(err)
		//return nil
	}

	message := comfoconnect.Message{
		Src:           src,
		Dst:           dst,
		Operation:     operation,
		RawMessage:    completeMessage,
		OperationType: operationType,
	}

	//log.Infof(message.Operation.Type.String())
	//log.Infof(fmt.Sprintf("%x", message.Src))
	//log.Infof(fmt.Sprintf("%x", message.Dst))
	//log.Infof(fmt.Sprintf("%v", operationType))
	log.Infof(message.String())

	//return completeMessage
	return message.Encode()

}

func ReadBytes(conn net.Conn, size int) ([]byte, error) {
	if size < 1 {
		err := errors.New(fmt.Sprintf("Invalid size: %d", size))
		return nil, err
	}
	var result []byte
	for {
		buffer := make([]byte, size)
		readLen, err := conn.Read(buffer)
		if err != nil {
			return nil, errors.Wrap(err, "reading from socket")
		}

		if readLen > 0 {
			size -= readLen
			result = append(result, buffer[:readLen]...)
		}

		if size == 0 {
			break
		}

		if size < 0 {
			return nil, errors.New("read too many bytes: size")
		}
	}
	return result, nil
}

func generateMetrics(message comfoconnect.Message) {

	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "generateMetrics",
	})

	switch message.Operation.Type.String() {
	case "CnRpdoNotificationType":
		conv := message.DecodePDO()
		log.Infof("Got RPDO: %s %v with value %f", reflect.TypeOf(conv), conv, conv.Tofloat64())
		metricsGauge.WithLabelValues(conv.GetID(), conv.GetDescription()).Set(conv.Tofloat64())
	case "CnAlarmNotificationType":
		log.Warnf("Got alarm notification: %v", message)
	}
	log.Debugf("called for %v", message)
}
