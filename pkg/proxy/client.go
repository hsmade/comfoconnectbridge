package proxy

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

type Client struct {
	IP         string
	uuid       []byte
	toRemote   chan comfoconnect.Message
	fromRemote chan comfoconnect.Message
	quit       chan bool
	exited     chan bool
}

func NewClient(ip string, macAddress []byte) *Client {
	_, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:56747", ip))
	if err != nil {
		logrus.Fatalf("failed to resolve address: %v", err)
	}

	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, macAddress...)

	return &Client{
		IP:         ip,
		uuid:       uuid,
		toRemote:   make(chan comfoconnect.Message),
		fromRemote: make(chan comfoconnect.Message),
	}
}

func (c Client) Run() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:56747", c.IP))
	if err != nil {
		return err
	}
	defer conn.Close()

	// register app
	// read message until response, check response
	// create session
	// read message until response, check response
	// for each PDO
	//     request PDO
	//     read message until response, check response

	for {
		select {
		case <-c.quit:
			logrus.Info("Shutting down tcp server")
			conn.Close()
			close(c.exited)
			return nil

		case m := <-c.toRemote:
			conn.Write(m.Encode())
		default:
			m, err := comfoconnect.GetMessageFromSocket(conn)
			c.fromRemote <- m
		}
	}
}
