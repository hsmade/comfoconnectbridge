package comfoconnect

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/proto"
)

type Session struct {
	IP   string
	Src  []byte
	Dst  []byte
	Conn net.Conn
}

func NewSession(ctx context.Context, wg *sync.WaitGroup, comfoConnectIP string, pin uint32, src []byte) (*Session, error) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "NewSession",
		"comfoConnectIP": comfoConnectIP,
		"pin": pin,
		"src": src,
	})

	// first ping the gateway to get its UUID
	dst, err := DiscoverGateway(comfoConnectIP)
	if err != nil {
		log.Errorf("failed to discover gateway: %v", err)
		return nil, errors.Wrap(err, "discovering gateway")
	}

	if src == nil {
		// create our UUID
		id := uuid.New()
		src = append(make([]byte, 16), id[:]...)
	}

	log.Debugf("set src=%x and dst=%x", src, dst)

	// connect to the gateway
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:56747", comfoConnectIP))
	if err != nil {
		return nil, err
	}
	log.Debugf("connected to %s", conn.RemoteAddr())

	// send a registration request
	deviceName := "Proxy"
	reference := uint32(1)
	operationType := proto.GatewayOperation_RegisterAppRequestType
	m := Message{
		Src: src,
		Dst: dst,
		Operation: proto.GatewayOperation{
			Type:      &operationType,
			Reference: &reference,
		},
		OperationType: &proto.RegisterAppRequest{
			Uuid:       src,
			Pin:        &pin,
			Devicename: &deviceName,
		},
	}

	log.Debugf("Writing RegisterAppRequest: %x", m.Encode())
	_, err = conn.Write(m.Encode())
	if err != nil {
		log.Errorf("failed to send RegisterAppRequest: %v", err)
		return nil, errors.Wrap(err, "sending RegisterAppRequest")
	}

	// receive the confirmation for the registration
	log.Debugf("receiving RegisterAppConfirm")
	m, err = GetMessageFromSocket(conn)
	if err != nil {
		log.Errorf("failed to receive RegisterAppConfirm: %v", err)
		return nil, errors.Wrap(err, "receiving RegisterAppConfirm")
	}
	if m.Operation.Type.String() != "RegisterAppConfirmType" {
		log.Errorf("invalid message type, expected RegisterAppConfirm but got: %v", m.String())
		return nil, errors.New(fmt.Sprintf("received invalid message type instead of RegisterAppConfirmType: %v", m.String()))
	}
	log.Debugf("received RegisterAppConfirm: %x", m.Encode())

	// send a start session request
	reference++
	operationType = proto.GatewayOperation_StartSessionRequestType
	_, err = conn.Write(Message{
		Src: src,
		Dst: dst,
		Operation: proto.GatewayOperation{
			Type:      &operationType,
			Reference: &reference,
		},
		OperationType: &proto.StartSessionRequest{},
	}.Encode())
	if err != nil {
		log.Errorf("failed to send StartSessionRequest: %v", err)
		return nil, errors.Wrap(err, "sending StartSessionRequest")
	}

	// receive the confirmation for the session
	m, err = GetMessageFromSocket(conn)
	if err != nil {
		log.Errorf("failed to receive StartSessionConfirm: %v", err)
		return nil, errors.Wrap(err, "receiving StartSessionConfirm")
	}
	if m.Operation.Type.String() != "StartSessionConfirmType" {
		log.Errorf("invalid message type, expected StartSessionConfirm but got: %v", m.String())
		return nil, errors.New(fmt.Sprintf("received invalid message type instead of StartSessionConfirmType: %v", m.String()))
	}

	s := Session{
		IP:   comfoConnectIP,
		Src:  src,
		Dst:  dst,
		Conn: conn,
	}

	log.Debug("starting keep-alive loop")
	wg.Add(1)
	go s.keepAlive(ctx, wg)
	return &s, nil
}

func (s *Session) keepAlive(ctx context.Context, wg *sync.WaitGroup) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Session",
		"method": "keepAlive",
	})

	ticker := time.NewTicker(5 * time.Second)
	reference := uint32(50)
	for {
		select {
		case <- ctx.Done():
			wg.Done()
			return
		case <-ticker.C:
			log.Debug("sending keep alive")
			operationType := proto.GatewayOperation_CnTimeRequestType
			m := Message{
				Src:           s.Src,
				Dst:           s.Dst,
				Operation:     proto.GatewayOperation{
					Type: &operationType,
					Reference: &reference,
				},
				OperationType: &proto.CnTimeRequest{},
			}
			_, err := s.Conn.Write(m.Encode())
			if err != nil {
				if errors.Cause(err) == io.EOF {
					log.Debug("Connection closed, stopping keepalives")
					return
				}
				log.Errorf("keepalive got error: %v", err)
			}
			reference ++
			if reference > 1024 {
				reference = 1
			}
		}
	}
}
// send a UDP packet to `ip` and expect a searchGatewayResponse with the uuid
func DiscoverGateway(ip string) (uuid []byte, err error) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "DiscoverGateway",
	})

	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:56747", ip))
	if err != nil {
		log.Errorf("could not resolve gateway address %s: %v", ip, err)
		return nil, errors.Wrap(err, fmt.Sprintf("resolving gateway address: %s", ip))
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		log.Errorf("could not connect to gateway address %s: %v", ip, err)
		return nil, errors.Wrap(err, fmt.Sprintf("connectinng to gateway address: %s", ip))
	}
	defer conn.Close()

	//conn.SetReadDeadline(time.Time{})

	_, err = conn.Write([]byte{0x0a, 0x00}) // wake up gateway
	if err != nil {
		log.Errorf("could write discovery packet to gateway address %s: %v", ip, err)
		return nil, errors.Wrap(err, fmt.Sprintf("writing discovery packet to gateway address: %s", ip))
	}

	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err != nil {
		log.Errorf("could read message from gateway address %s: %v", ip, err)
		return nil, errors.Wrap(err, fmt.Sprintf("reading message from gateway address: %s", ip))
	}
	response := proto.SearchGatewayResponse{}
	err = response.XXX_Unmarshal(buf[2:])
	//if err != nil {
	//	return nil, errors.Wrap(err, fmt.Sprintf("marshalling message from gateway address: %s", ip))
	//}

	return response.Uuid, nil
}

func (s *Session) Close() {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Session",
		"method": "Close",
	})
	// send a start session request
	defer s.Conn.Close()

	log.Debug("sending CloseSessionRequest")
	reference := uint32(1)
	operationType := proto.GatewayOperation_CloseSessionRequestType
	_, _ = s.Conn.Write(Message{
		Src: s.Src,
		Dst: s.Dst,
		Operation: proto.GatewayOperation{
			Type:      &operationType,
			Reference: &reference,
		},
		OperationType: &proto.CloseSessionRequest{},
	}.Encode())
}

func (s *Session) Receive() (Message, error) {
	s.Conn.SetReadDeadline(time.Now().Add(time.Millisecond * 300))
	m, err := GetMessageFromSocket(s.Conn)
	return m, err
}

func (s *Session) Send(message Message) error {
	span := opentracing.GlobalTracer().StartSpan("comfoconnect.Session.Send", opentracing.ChildOf(message.Span.Context()))
	defer span.Finish()
	SpanSetMessage(span, message)
	len, err := s.Conn.Write(message.Encode())
	span.SetTag("written", len)
	return err
}
