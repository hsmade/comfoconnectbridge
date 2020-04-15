package proxy

import (
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Proxy struct {
	client    *Client
	uuid      []byte
	listener  *Listener
	toGateway chan comfoconnect.Message
	quit      chan bool
	exited    chan bool
}

func NewProxy(comfoConnectIP string, myMacAddress []byte) *Proxy {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "NewProxy",
	})

	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, myMacAddress...)

	toGateway := make(chan comfoconnect.Message)
	log.Info("creating new listener")
	l := NewListener(toGateway)
	log.Info("creating new cient")
	c := NewClient(comfoConnectIP, myMacAddress, toGateway)

	p := Proxy{
		client:    c,
		listener:  l,
		uuid:      uuid,
		toGateway: toGateway,
	}

	return &p
}

func (p Proxy) Run() {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Proxy",
		"method": "Run",
	})

	log.Info("starting new cient")
	go p.client.Run()
	log.Info("starting new listener")
	go p.listener.Run()

	for {
		select {
		case <-p.quit:
			log.Info("Shutting down proxy server")
			p.client.Stop()
			p.listener.Stop()
			close(p.exited)
			return
		case message := <-p.toGateway:
			log.Debugf("received a message: %v", message)
			generateMetrics(message)
			message.Src = p.uuid
			log.Debugf("forwarding message to gateway: %v", message)
			p.client.toGateway <- message
		case message := <-p.client.fromGateway:
			log.Debugf("received a message from gateway: %v", message)
			generateMetrics(message)
			for _, app := range p.listener.apps {
				log.Debugf("copying message from gateway to app(%s/%x):%v", app.conn.RemoteAddr().String(), app.uuid)
				message.Dst = app.uuid
				err := app.Write(message)
				if err != nil {
					log.Errorf("error while copying message from gateway to app(%s/%x):%v", app.conn.RemoteAddr().String(), app.uuid, err)
				}
			}
		}
	}
}

func (p *Proxy) Stop() {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Proxy",
		"method": "Stop",
	})

	log.Debug("Stopping")
	close(p.quit)
	<-p.exited
	log.Info("Stopped")
}

func generateMetrics(message comfoconnect.Message) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Proxy",
		"method": "generateMetrics",
	})

	log.Debugf("called for %v", message)
}
