package proxy

import "github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"

type Proxy struct {
	client   *Client
	uuid     []byte
	toClient chan comfoconnect.Message
	apps     map[string]*App
}

func (p Proxy) Run() {
	p.client = NewClient("x.x.x.x", []byte{})
	go p.client.Run()

	for {
		select {
		case message := <-p.toClient:
			generateMetrics(message)
			message.Src = p.uuid
			p.client.toRemote <- message
		case message := <-p.client.fromRemote:
			generateMetrics(message)
			for _, app := range p.apps {
				message.Dst = app.uuid
				app.Write(message)
			}
		}
	}
}

func generateMetrics(message comfoconnect.Message) {

}
