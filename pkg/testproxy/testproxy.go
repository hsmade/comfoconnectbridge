package testproxy

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Proxy struct {
	ComfoConnect string
	listener     *net.TCPListener
	quit         chan bool
	exited       chan bool
	clients      []*net.Conn
}

func NewProxy(comfoConnect string) *Proxy {
	addr, err := net.ResolveTCPAddr("tcp4", ":56747")
	if err != nil {
		logrus.Fatalf("failed to resolve address: %v", err)
	}

	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		logrus.Fatalf("failed to create listener: %v", err)
	}

	return &Proxy{
		ComfoConnect: comfoConnect, // ip:port
		quit:         make(chan bool),
		exited:       make(chan bool),
		listener:     listener,
	}
}

func (p *Proxy) Run() {
	logrus.Debug("Starting new Proxy listener")
	var handlers sync.WaitGroup
	for {
		select {
		case <-p.quit:
			logrus.Info("Shutting down tcp server")
			p.listener.Close()
			handlers.Wait()
			close(p.exited)
			return

		default:
			err := p.listener.SetDeadline(time.Now().Add(time.Second * 1))
			if err != nil {
				logrus.Errorf("failed to set read deadline: %v", err)
				continue
			}

			// logrus.Debug("waiting for new connections")
			conn, err := p.listener.Accept()
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
					err := p.handleClient(conn)
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

func (p *Proxy) copy(from, to net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	for {

		message, err := comfoconnect.GetMessageFromSocket(from)
		if err != nil {
			if errors.Cause(err) == io.EOF {
				return
			}
			logrus.Errorf("src: %s, dst:%s, err: %v", from.RemoteAddr(), to.RemoteAddr(), err)
			continue
		}

		logrus.Infof("received message: %s", *message)
		writeLen, err := to.Write(message.RawMessage)
		logrus.Debugf("wrote %d: %v", writeLen, err)
	}
}

func (p *Proxy) handleClient(conn net.Conn) error {
	defer conn.Close()
	remote, err := net.Dial("tcp", p.ComfoConnect)
	if err != nil {
		return err
	}
	defer remote.Close()

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go p.copy(remote, conn, wg)
	go p.copy(conn, remote, wg)
	wg.Wait()

	return nil
}

func (p *Proxy) Stop() {
	logrus.Info("Stopping tcp server")
	close(p.quit)
	<-p.exited
	logrus.Info("Stopped tcp server")
}
