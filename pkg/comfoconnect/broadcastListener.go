package comfoconnect

import (
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
)

type BroadcastListener struct {
	ResponseIP string
	myMAC      []byte
	listener   *net.UDPConn
	quit       chan bool
	exited     chan bool
}

func NewBroadcastListener(ip string, mac []byte) *BroadcastListener {
	addr, err := net.ResolveUDPAddr("udp4", ":56747")
	if err != nil {
		logrus.Fatalf("failed to resolve address: %v", err)
	}

	listener, err := net.ListenUDP("udp4", addr)
	if err != nil {
		logrus.Fatalf("failed to create listener: %v", err)
	}

	l := BroadcastListener{
		ResponseIP: ip,
		myMAC:      mac,
		listener:   listener,
		quit:       make(chan bool),
		exited:     make(chan bool),
	}

	return &l
}

func (l *BroadcastListener) Run() {
	logrus.Debug("Starting new BroadcastListener")
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
			err := l.listener.SetReadDeadline(time.Now().Add(time.Second * 5))
			if err != nil {
				logrus.Errorf("failed to set read deadline: %v", err)
				continue
			}

			b := make([]byte, 2)
			logrus.Debug("waiting for UDP broadcast")
			_, addr, err := l.listener.ReadFromUDP(b)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				logrus.Errorf("failed to accept connection: %v", err)
				continue
			}
			logrus.Debugf("Received: %v from %v", b, addr.String())

			if addr != nil {
				handlers.Add(1)
				go func() {
					err := l.handleConnection(addr)
					if err != nil {
						logrus.Errorf("failed to handle connection: %v", err)
					}
					handlers.Done()
				}()
			}
		}
	}
}

func (l *BroadcastListener) handleConnection(addr *net.UDPAddr) error {
	logrus.Debug("writing searchGatewayResponse")
	_, err := l.listener.WriteToUDP(CreateSearchGatewayResponse(l.ResponseIP, l.myMAC), addr)
	if err != nil {
		logrus.Errorf("Failed to respond to SearchGatewayRequest: %v", err)
		return errors.Wrap(err, "responding to SearchGatewayRequest")
	}
	return nil
}

func (l *BroadcastListener) Stop() {
	logrus.Info("Stopping udp listener")
	close(l.quit)
	<-l.exited
	logrus.Info("Stopped udp listener")
}
