package comfoconnect

import (
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/hsmade/comfoconnectbridge/pkg/helpers"
)

type BroadcastListener struct {
	ResponseIP string
	myUUID     []byte
	listener   *net.UDPConn
	quit       chan bool
	exited     chan bool
}

func NewBroadcastListener(myIP string, myUUID []byte) *BroadcastListener {
	addr, err := net.ResolveUDPAddr("udp4", ":56747")
	helpers.PanicOnError(err)

	listener, err := net.ListenUDP("udp4", addr)
	helpers.PanicOnError(err)

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
	helpers.StackLogger().Debug("Starting")
	var handlers sync.WaitGroup
	for {
		select {
		case <-l.quit:
			helpers.StackLogger().Info("Shutting down")
			//l.listener.Close()
			handlers.Wait()
			close(l.exited)
			return

		default:
			err := l.listener.SetReadDeadline(time.Now().Add(time.Second * 5))
			if err != nil {
				helpers.StackLogger().Errorf("failed to set read deadline: %v", err)
				continue
			}

			b := make([]byte, 2)
			//helpers.StackLogger().Debug("waiting for UDP broadcast")
			_, addr, err := l.listener.ReadFromUDP(b)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				helpers.StackLogger().Errorf("failed to accept connection: %v", err)
				continue
			}
			helpers.StackLogger().Debugf("Received: %v from %v", b, addr.String())

			if addr != nil {
				handlers.Add(1)
				go func() {
					err := l.handleConnection(addr)
					if err != nil {
						helpers.StackLogger().Errorf("failed to handle connection: %v", err)
					}
					handlers.Done()
				}()
			}
		}
	}
}

func (l *BroadcastListener) handleConnection(addr *net.UDPAddr) error {
	helpers.StackLogger().Debugf("writing searchGatewayResponse: ip=%s uuid=%s", l.ResponseIP, l.myUUID)
	_, err := l.listener.WriteToUDP(CreateSearchGatewayResponse(l.ResponseIP, l.myUUID), addr)
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "responding to SearchGatewayRequest"))
	}
	return nil
}

func (l *BroadcastListener) Stop() {
	helpers.StackLogger().Debugf("Stopping")
	_ = l.listener.Close()
	close(l.quit)
	<-l.exited
	helpers.StackLogger().Info("Stopped")
}
