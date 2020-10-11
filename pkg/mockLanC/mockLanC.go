package mockLanC

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/hsmade/comfoconnectbridge/pb"
	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/helpers"
)

type MockLanC struct {
	myIP           string // IP to bind to and return with on broadcast requests
	comfoconnectIP string
	listener       *net.TCPListener
	quit           chan bool
	exited         chan bool
}

func NewMockLanC(myIP, comfoconnectIP string) *MockLanC {
	addr, err := net.ResolveTCPAddr("tcp4", ":56747")
	if err != nil {
		helpers.PanicOnError(errors.Wrap(err, "failed to resolve address"))
	}

	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		helpers.PanicOnError(errors.Wrap(err, "failed to create listener"))
	}

	b := MockLanC{
		myIP:           myIP,
		comfoconnectIP: comfoconnectIP,
		listener:       listener,
		quit:           make(chan bool),
		exited:         make(chan bool),
	}

	return &b
}

func (m *MockLanC) Run() {
	helpers.StackLogger().Debug("Starting new MockLanC")
	var handlers sync.WaitGroup
	for {
		select {
		case <-m.quit:
			helpers.StackLogger().Info("Shutting down tcp server")
			m.listener.Close()
			handlers.Wait()
			close(m.exited)
			return

		default:
			err := m.listener.SetDeadline(time.Now().Add(time.Second * 1))
			if err != nil {
				helpers.StackLogger().Errorf("failed to set accept deadline: %v", err)
				continue
			}

			// helpers.StackLogger().Debug("waiting for new connections")
			conn, err := m.listener.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				helpers.StackLogger().Errorf("failed to accept connection: %v", err)
				continue
			}
			handlers.Add(1)
			go func() {
				for {
					err := m.handleClient(conn)
					if err != nil {
						helpers.StackLogger().Errorf("failed to handle connection: %v", err)
						break
					}
				}
				handlers.Done()
			}()
		}
	}
}

func (m *MockLanC) handleClient(conn net.Conn) error {
	helpers.StackLogger().Debugf("handling connection from %v", conn.RemoteAddr())
	defer conn.Close()

	for {
		message, err := comfoconnect.NewMessageFromSocket(conn)
		if err != nil {
			if err, ok := errors.Cause(err).(net.Error); ok && err.Timeout() {
				// this is a timeout, which just means there is no data (yet)
				continue
			}

			if errors.Cause(err) == io.EOF {
				helpers.StackLogger().Warnf("client %s closed connection", conn.RemoteAddr())
				return errors.Wrap(err, "tried to read from a closed connection") // FIXME: not an error?
			}

			helpers.StackLogger().Errorf("failed to parse this message from: %s: %v", conn.RemoteAddr(), err)
			continue
		}

		helpers.StackLogger().Infof("got a message from: %s: %v", conn.RemoteAddr(), message)

		switch message.Operation.Type.String() {
		//case "StartSessionRequestType":
		//	m.respond(conn, message.CreateResponse(proto.GatewayOperation_OK))
		case "StartSessionRequestType":
			m.respond(conn, message.CreateResponse(nil, pb.GatewayOperation_OK))

			i := uint32(1)
			mode := pb.CnNodeNotification_NODE_NORMAL
			a := pb.CnNodeNotification{
				NodeId:    &i,
				ProductId: &i,
				ZoneId:    &i,
				Mode:      &mode,
			}

			m.respond(conn, message.CreateCustomResponse(nil, pb.GatewayOperation_CnNodeNotificationType, &a))
			i48 := uint32(48)
			i5 := uint32(5)
			i255 := uint32(255)
			mode = pb.CnNodeNotification_NODE_NORMAL
			a = pb.CnNodeNotification{
				NodeId:    &i48,
				ProductId: &i5,
				ZoneId:    &i255,
				Mode:      &mode,
			}
			m.respond(conn, message.CreateCustomResponse(nil, pb.GatewayOperation_CnNodeNotificationType, &a))
		default:
			m.respond(conn, message.CreateResponse(nil, -1))
		}
	}
}

func (m *MockLanC) Stop() {
	helpers.StackLogger().Info("Stopping tcp server")
	close(m.quit)
	<-m.exited
	helpers.StackLogger().Info("Stopped tcp server")
}

func (m *MockLanC) respond(conn net.Conn, data []byte) error {
	helpers.StackLogger().Debugf("responding to %v with %x", conn.RemoteAddr(), data)
	_, err := conn.Write(data)
	// FIXME error logging
	return err
}
