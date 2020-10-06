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
	myUUID     []byte
	listener   *net.UDPConn
	quit       chan bool
	exited     chan bool
}

func NewBroadcastListener(myIP string, myUUID []byte) *BroadcastListener {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"method": "NewBroadcastListener",
	})

	addr, err := net.ResolveUDPAddr("udp4", ":56747")
	if err != nil {
		log.Fatalf("failed to resolve address: %v", err)
	}

	listener, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}

	l := BroadcastListener{
		ResponseIP: myIP,
		myUUID:     myUUID,
		listener:   listener,
		quit:       make(chan bool),
		exited:     make(chan bool),
	}

	return &l
}

func (l *BroadcastListener) Run() {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "BroadcastListener",
		"method": "Run",
	})

	log.Debug("Starting")
	var handlers sync.WaitGroup
	for {
		select {
		case <-l.quit:
			log.Info("Shutting down")
			//l.listener.Close()
			handlers.Wait()
			close(l.exited)
			return

		default:
			err := l.listener.SetReadDeadline(time.Now().Add(time.Second * 5))
			if err != nil {
				log.Errorf("failed to set read deadline: %v", err)
				continue
			}

			b := make([]byte, 2)
			//log.Debug("waiting for UDP broadcast")
			_, addr, err := l.listener.ReadFromUDP(b)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				log.Errorf("failed to accept connection: %v", err)
				continue
			}
			log.Debugf("Received: %v from %v", b, addr.String())

			if addr != nil {
				handlers.Add(1)
				go func() {
					err := l.handleConnection(addr)
					if err != nil {
						log.Errorf("failed to handle connection: %v", err)
					}
					handlers.Done()
				}()
			}
		}
	}
}

func (l *BroadcastListener) handleConnection(addr *net.UDPAddr) error {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "BroadcastListener",
		"method": "handleConnection",
	})

	log.Debugf("writing searchGatewayResponse: ip=%s uuid=%s", l.ResponseIP, l.myUUID)
	_, err := l.listener.WriteToUDP(CreateSearchGatewayResponse(l.ResponseIP, l.myUUID), addr)
	if err != nil {
		log.Errorf("Failed to respond to SearchGatewayRequest: %v", err)
		return errors.Wrap(err, "responding to SearchGatewayRequest")
	}
	return nil
}

func (l *BroadcastListener) Stop() {
	log := logrus.WithFields(logrus.Fields{
		"module": "proxy",
		"object": "BroadcastListener",
		"method": "Stop",
	})

	log.Debugf("Stopping")
	_ = l.listener.Close()
	close(l.quit)
	<-l.exited
	log.Info("Stopped")
}
