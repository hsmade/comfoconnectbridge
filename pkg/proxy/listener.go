package proxy

import (
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Listener struct {
	listener *net.TCPListener
	quit     chan bool
	exited   chan bool
	proxy    *Proxy
}

func NewListener(proxy *Proxy) *Listener {
	addr, err := net.ResolveTCPAddr("tcp4", ":56747")
	if err != nil {
		logrus.Fatalf("failed to resolve address: %v", err)
	}

	listener, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		logrus.Fatalf("failed to create listener: %v", err)
	}

	return &Listener{
		quit:     make(chan bool),
		exited:   make(chan bool),
		listener: listener,
		proxy:    proxy,
	}
}

func (l *Listener) Run() {
	logrus.Debug("Starting new Proxy listener")
	var handlers sync.WaitGroup
	for {
		select {
		case <-l.quit:
			logrus.Info("Shutting down tcp server")
			l.listener.Close()
			handlers.Wait()
			close(l.exited)
			return

		default:
			err := l.listener.SetDeadline(time.Now().Add(time.Second * 5))
			if err != nil {
				logrus.Errorf("failed to set read deadline: %v", err)
				continue
			}

			logrus.Debug("waiting for new connections")
			conn, err := l.listener.Accept()
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
					app := App{conn: conn}
					l.proxy.apps[conn.RemoteAddr().String()] = &app
					err := app.HandleConnection(l.proxy.toClient)
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

type App struct {
	uuid []byte
	conn net.Conn
}

func (a *App) HandleConnection(remote chan comfoconnect.Message) error {
	for {
		//read message
		message, err := comfoconnect.GetMessageFromSocket(a.conn)
		switch message.Operation.Type.String() {
		//case "RegisterAppRequestType": answer with confirm; store uuid (src)
		//case "StartSessionRequestType": answer with confirm and 2 node-notifications
		default:
			remote <- message
		}
	}
}

func (a *App) Write(message comfoconnect.Message) error {
	_, err := a.conn.Write(message.Encode())
	return err
}
