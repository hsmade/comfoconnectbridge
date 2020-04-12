package bridge

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	//"github.com/hsmade/comfoconnectbridge/proto"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/proto"
)

type Bridge struct {
	myIP           string // IP to bind to and return with on broadcast requests
	comfoconnectIP string
	listener       *net.TCPListener
	quit           chan bool
	exited         chan bool
}

func NewBridge(myIP, comfoconnectIP string) *Bridge {
	addr, err := net.ResolveTCPAddr("tcp4", ":56747")
	if err != nil {
		logrus.Fatalf("failed to resolve address: %v", err)
	}

	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		logrus.Fatalf("failed to create listener: %v", err)
	}

	b := Bridge{
		myIP:           myIP,
		comfoconnectIP: comfoconnectIP,
		listener:       listener,
		quit:           make(chan bool),
		exited:         make(chan bool),
	}

	return &b
}

func (b *Bridge) Run() {
	logrus.Debug("Starting new Bridge")
	var handlers sync.WaitGroup
	for {
		select {
		case <-b.quit:
			logrus.Info("Shutting down tcp server")
			b.listener.Close()
			handlers.Wait()
			close(b.exited)
			return

		default:
			err := b.listener.SetDeadline(time.Now().Add(time.Second * 5))
			if err != nil {
				logrus.Errorf("failed to set accept deadline: %v", err)
				continue
			}

			logrus.Debug("waiting for new connections")
			conn, err := b.listener.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				logrus.Errorf("failed to accept connection: %v", err)
				continue
			}
			handlers.Add(1)
			go func() {
				for {
					err := b.handleClient(conn)
					if err != nil {
						logrus.Errorf("failed to handle connection: %v", err)
						break
					}
				}
				handlers.Done()
			}()
		}
	}
}

func (b *Bridge) handleClient(conn net.Conn) error {
	logrus.Debugf("handling connection from %v", conn.RemoteAddr())
	defer conn.Close()

	for {
		message, err := comfoconnect.GetMessageFromSocket(conn)
		if err != nil {
			if err, ok := errors.Cause(err).(net.Error); ok && err.Timeout() {
				// this is a timeout, which just means there is no data (yet)
				continue
			}

			if errors.Cause(err) == io.EOF {
				logrus.Warnf("client %s closed connection", conn.RemoteAddr())
				return errors.Wrap(err, "tried to read from a closed connection") // FIXME: not an error?
			}

			logrus.Errorf("failed to parse this message from: %s: %v", conn.RemoteAddr(), err)
			continue
		}

		logrus.Infof("got a message from: %s: %v", conn.RemoteAddr(), message)

		switch message.Operation.Type.String() {
		//case "StartSessionRequestType":
		//	b.respond(conn, message.CreateResponse(proto.GatewayOperation_OK))
		case "StartSessionRequestType":
			b.respond(conn, message.CreateResponse(proto.GatewayOperation_OK))

			i := uint32(1)
			mode := proto.CnNodeNotification_NODE_NORMAL
			a := proto.CnNodeNotification{
				NodeId:               &i,
				ProductId:            &i,
				ZoneId:               &i,
				Mode:                 &mode,
			}

			b.respond(conn, message.CreateCustomResponse(proto.GatewayOperation_CnNodeNotificationType, &a))
			i48 := uint32(48)
			i5 := uint32(5)
			i255 := uint32(255)
			mode = proto.CnNodeNotification_NODE_NORMAL
			a = proto.CnNodeNotification{
				NodeId:               &i48,
				ProductId:            &i5,
				ZoneId:               &i255,
				Mode:                 &mode,
			}
			b.respond(conn, message.CreateCustomResponse(proto.GatewayOperation_CnNodeNotificationType, &a))
		default:
			b.respond(conn, message.CreateResponse(-1))
		}
	}
}

func (b *Bridge) Stop() {
	logrus.Info("Stopping tcp server")
	close(b.quit)
	<-b.exited
	logrus.Info("Stopped tcp server")
}

func (b *Bridge) respond(conn net.Conn, data []byte) error {
	logrus.Debugf("responding to %v with %x", conn.RemoteAddr(), data)
	_, err := conn.Write(data)
	// FIXME error logging
	return err
}
