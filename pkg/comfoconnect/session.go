package comfoconnect

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/hsmade/comfoconnectbridge/pb"
	"github.com/hsmade/comfoconnectbridge/pkg/helpers"
)

type Session struct {
	IP   string
	Src  []byte
	Dst  []byte
	Conn net.Conn
}

func NewSession(ctx context.Context, wg *sync.WaitGroup, comfoConnectIP string, pin uint32, src []byte) (*Session, error) {
	helpers.StackLogger().Infof("Starting new sessions with src:%x", src)
	// first ping the gateway to get its UUID
	dst, err := DiscoverGateway(comfoConnectIP)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "discovering gateway"))
	}

	if src == nil {
		// create our UUID
		id := uuid.New()
		src = append(make([]byte, 16), id[:]...)
	}

	helpers.StackLogger().Debugf("set src=%x and dst=%x", src, dst)

	// connect to the gateway
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:56747", comfoConnectIP))
	if err != nil {
		return nil, err
	}
	helpers.StackLogger().Debugf("connected to %s", conn.RemoteAddr())

	// send a registration request
	deviceName := "Proxy"
	reference := uint32(1)
	operationType := pb.GatewayOperation_RegisterAppRequestType
	m := &Message{
		Src: src,
		Dst: dst,
		Operation: &pb.GatewayOperation{
			Type:      &operationType,
			Reference: &reference,
		},
		OperationType: &pb.RegisterAppRequest{
			Uuid:       src,
			Pin:        &pin,
			Devicename: &deviceName,
		},
	}

	helpers.StackLogger().Debugf("Writing RegisterAppRequest: %x", m.Encode())
	_, err = conn.Write(m.Encode())
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "sending RegisterAppRequest"))
	}

	// receive the confirmation for the registration
	helpers.StackLogger().Debugf("receiving RegisterAppConfirm")
	m, err = NewMessageFromSocket(conn)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "receiving RegisterAppConfirm"))
	}
	if m.Operation.Type.String() != "RegisterAppConfirmType" {
		return nil, helpers.LogOnError(errors.New(fmt.Sprintf("received invalid message type instead of RegisterAppConfirmType: %v", m.String())))
	}
	helpers.StackLogger().Debugf("received RegisterAppConfirm: %x", m.Encode())

	// send a start session request
	reference++
	operationType = pb.GatewayOperation_StartSessionRequestType
	_, err = conn.Write(Message{
		Src: src,
		Dst: dst,
		Operation: &pb.GatewayOperation{
			Type:      &operationType,
			Reference: &reference,
		},
		OperationType: &pb.StartSessionRequest{},
	}.Encode())
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "sending StartSessionRequest"))
	}

	// receive the confirmation for the session
	m, err = NewMessageFromSocket(conn)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "receiving StartSessionConfirm"))
	}
	if m.Operation.Type.String() != "StartSessionConfirmType" {
		return nil, helpers.LogOnError(errors.New(fmt.Sprintf("received invalid message type instead of StartSessionConfirmType: %v", m.String())))
	}

	s := Session{
		IP:   comfoConnectIP,
		Src:  src,
		Dst:  dst,
		Conn: conn,
	}

	helpers.StackLogger().Debug("starting keep-alive loop")
	wg.Add(1)
	go s.keepAlive(ctx, wg)
	return &s, nil
}

func (s *Session) keepAlive(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(5 * time.Second)
	reference := uint32(50)
	for {
		select {
		case <-ctx.Done():
			wg.Done()
			return
		case <-ticker.C:
			helpers.StackLogger().Debug("sending keep alive")
			operationType := pb.GatewayOperation_CnTimeRequestType
			m := Message{
				Src: s.Src,
				Dst: s.Dst,
				Operation: &pb.GatewayOperation{
					Type:      &operationType,
					Reference: &reference,
				},
				OperationType: &pb.CnTimeRequest{},
			}
			_, err := s.Conn.Write(m.Encode())
			if err != nil {
				if errors.Cause(err) == io.EOF {
					helpers.StackLogger().Debug("Connection closed, stopping keepalives")
					return
				}
				helpers.StackLogger().Errorf("keepalive got error: %v", err)
			}
			reference++
			if reference > 1024 {
				reference = 1
			}
		}
	}
}

// send a UDP packet to `ip` and expect a searchGatewayResponse with the uuid
func DiscoverGateway(ip string) (uuid []byte, err error) {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:56747", ip))
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, fmt.Sprintf("resolving gateway address: %s", ip)))
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, fmt.Sprintf("connectinng to gateway address: %s", ip)))
	}
	defer conn.Close()

	//conn.SetReadDeadline(time.Time{})

	_, err = conn.Write([]byte{0x0a, 0x00}) // wake up gateway
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, fmt.Sprintf("writing discovery packet to gateway address: %s", ip)))
	}

	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, fmt.Sprintf("reading message from gateway address: %s", ip)))
	}
	response := pb.SearchGatewayResponse{}
	err = proto.Unmarshal(buf[2:], &response)
	//err = response.XXX_Unmarshal(buf[2:])
	//if err != nil {
	//	return nil, errors.Wrap(err, fmt.Sprintf("marshalling message from gateway address: %s", ip))
	//}

	return response.Uuid, helpers.LogOnError(err)
}

func (s *Session) Close() {
	// send a start session request
	defer s.Conn.Close()

	helpers.StackLogger().Debug("sending CloseSessionRequest")
	reference := uint32(1)
	operationType := pb.GatewayOperation_CloseSessionRequestType
	_, _ = s.Conn.Write(Message{
		Src: s.Src,
		Dst: s.Dst,
		Operation: &pb.GatewayOperation{
			Type:      &operationType,
			Reference: &reference,
		},
		OperationType: &pb.CloseSessionRequest{},
	}.Encode())
}

func (s *Session) Receive() (*Message, error) {
	_ = s.Conn.SetReadDeadline(time.Now().Add(time.Millisecond * 300))
	m, err := NewMessageFromSocket(s.Conn)
	return m, helpers.LogOnError(err)
}

func (s *Session) Send(message Message) error {
	span := opentracing.GlobalTracer().StartSpan("comfoconnect.Session.Send", opentracing.ChildOf(message.Span.Context()))
	defer span.Finish()
	SpanSetMessage(span, message)
	length, err := s.Conn.Write(message.Encode())
	helpers.StackLogger().Infof("Wrote %d bytes to gateway. err:%v bytes:%x message:%v", length, err, message.Encode(), message)
	span.SetTag("written", length)
	return helpers.LogOnError(err)
}
