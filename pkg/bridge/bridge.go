package bridge

import (
	"encoding/binary"
	"fmt"
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
	myIP             string // IP to bind to and return with on broadcast requests
	comfoconnectIP   string
	listener         *net.TCPListener
	quit             chan bool
	exited           chan bool
	toComfoConnect   chan []byte
	fromComfoConnect chan []byte
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
		myIP:             myIP,
		comfoconnectIP:   comfoconnectIP,
		listener:         listener,
		quit:             make(chan bool),
		exited:           make(chan bool),
		toComfoConnect:   make(chan []byte),
		fromComfoConnect: make(chan []byte),
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
				err := b.handleConnection(conn)
				if err != nil {
					logrus.Errorf("failed to handle connection: %v", err)
				}
				handlers.Done()
			}()
		}
	}
}

func (b *Bridge) handleConnection(conn net.Conn) error {
	logrus.Debugf("handling connection from %v", conn.RemoteAddr())
	defer conn.Close()
	// get the messageSize of the messageBuffer
	var readBuffer []byte
	readBytes := 0
	for {
		b := make([]byte, 4 - readBytes)
		readLen, err := conn.Read(b)
		if err != nil {
			msg := fmt.Sprintf("failed to read message. Got messageSize=%d and err=%v", readLen, err)
			logrus.Error(msg)
			return errors.New(msg)
		}

		//logrus.Debugf("got %d bytes: %x", readLen, b)
		readBytes += readLen
		if readLen > 0 {
			readBuffer = append(readBuffer, b[:readLen]...)
			//logrus.Debugf("appending %d bytes (%x), result: %x", readLen, b, readBuffer)
		}

		if readBytes >= 4 {
			break
		}
	}

	messageSize := binary.BigEndian.Uint32(readBuffer)
	//logrus.Debugf("got a message with size: %d", messageSize)
	rest := messageSize

	// get the rest of the messageBuffer
	messageBuffer := readBuffer
	for {
		buf := make([]byte, 1024)
		reqLen, err := conn.Read(buf)
		if err != nil {
			return errors.Wrap(err, "error reading connection")
		}
		//logrus.Debugf("received(%d): %v", reqLen, buf[:reqLen+1])
		if uint32(reqLen) > rest {
			msg := fmt.Sprintf("expected max %d, but got %d", rest, reqLen)
			logrus.Error(msg)
			return errors.New(msg)
		}
		rest = rest - uint32(reqLen)
		messageBuffer = append(messageBuffer, buf[:reqLen]...)
		//logrus.Debugf("message is now: %x", messageBuffer)
		if rest <= 0 {
			break
		}
	}
	logrus.Debugf("message final: %x", messageBuffer)
	message := comfoconnect.NewMessage(messageBuffer)
	logrus.Debugf("got a message with type: %v: %v", message.Cmd.Type.String(), message)

	switch message.Cmd.Type.String() {
	case "RegisterAppRequestType":
		cmd := proto.RegisterAppConfirm{}
		data, _ := cmd.XXX_Marshal(nil, false)
		logrus.Debugf("responding with RegisterAppConfirm: %x", data)
		b.respond(conn, message.CreateResponse(data, proto.GatewayOperation_RegisterAppConfirmType, -1))
	case "StartSessionRequestType":
		//name := "OnePlus GM1913"
		//resumed := false
		cmd := proto.StartSessionConfirm{
		//Devicename: &name,
		//Resumed: &resumed,
		}
		data, _ := cmd.XXX_Marshal(nil, false)
		logrus.Debugf("responding with StartSessionConfirm: %x", data)
		b.respond(conn, message.CreateResponse(data, proto.GatewayOperation_StartSessionConfirmType, proto.GatewayOperation_OK))
	case "CloseSessionRequestType":
		cmd := proto.CloseSessionConfirm{}
		data, _ := cmd.XXX_Marshal(nil, false)
		logrus.Debugf("responding with CloseSessionConfirm: %x", data)
		b.respond(conn, message.CreateResponse(data, proto.GatewayOperation_CloseSessionConfirmType, -1))
	}
	// TODO: implementation
	// respond to RegisterAppRequest with RegisterAppConfirm and don't forward
	// respond to StartSessionRequest with StartSessionConfirm and don't forward

	// forward the message to our local comfoconnect first.
	// if it's a session request, open up a new session so we forward message coming from comfoconnect back to them

	return nil
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
