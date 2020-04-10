package comfoconnect

import (
	"encoding/binary"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/proto"
)

type UnMarshaller interface {
	XXX_Unmarshal([]byte) error
}

type TypeObject interface {
	Type() proto.GatewayOperation_OperationType
	XXX_Marshall([]byte, bool) ([]byte, error)
}

type Message struct {
	length uint32
	src    []byte
	dst    []byte
	Cmd    proto.GatewayOperation
	msg    []byte
	Operation interface{}
}

func NewMessage(b []byte) Message {
	cmd := proto.GatewayOperation{}
	cmd.XXX_Unmarshal(b[38:42]) // we assume here that this is always a gateway operation, else we'd have to read the cmd length from [36:38]

	var op UnMarshaller
	switch cmd.Type.String() {
	case "RegisterAppRequestType": op = &proto.RegisterAppRequest{}
	case "StartSessionRequestType": op = &proto.StartSessionRequest{}
	}

	op.XXX_Unmarshal(b[42:])

	m := Message{
		length: binary.BigEndian.Uint32(b[:4]),
		src:    b[4:20],
		dst:    b[20:36],
		Cmd:    cmd,
		msg:    b[42:],
		Operation: op,
	}

	fmt.Printf("%v", m)

	return m
	//Cmd := proto.GatewayOperation{}
	//Cmd.XXX_Unmarshal(b[34:38])
	//fmt.Printf("%s\n", Cmd.String())
	//ding := proto.RegisterAppRequest{}
	//ding.XXX_Unmarshal(b[38:])
	//fmt.Printf("%s\n", ding.String())

}

func (m Message) String() string {
	return fmt.Sprintf("src=%x; dst=%x; Cmd type=%v; msg=%x", m.src, m.dst, m.Cmd.Type.String(), m.msg)
}

func (m Message) CreateResponse(msg []byte, operationType proto.GatewayOperation_OperationType, result proto.GatewayOperation_GatewayResult) []byte {
	cmd := proto.GatewayOperation{
		Type: &operationType,
		Reference: m.Cmd.Reference,
		Result: &result,
	}
	if result == -1 {
		cmd.Result = nil
	}

	cmdBytes, _ := cmd.XXX_Marshal(nil, false)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(msg))) // msg length
	logrus.Debugf("size message(%d): %x", len(msg), b)
	b = append(b, m.src...)
	logrus.Debugf("src message: %x", b)
	b = append(b, m.dst...)
	logrus.Debugf("dst message: %x", b)
	b = append(b, []byte{0x00, 0x04}...) // cmd length, hardcoded to 4
	logrus.Debugf("len message: %x", b)
	b = append(b, cmdBytes...) // gatewayOperation
	logrus.Debugf("cmd message: %x", b)
	b = append(b, msg...)
	logrus.Debugf("msg message: %x", b)

	return b
}

// 000000122ffbbe53b6234079aa7c8a6872da115a000000000025101080017085c2b78ca00004 08351000 200e0a0e4f6e65506c757320474d313931331000
// 000000002ffbbe53b6234079aa7c8a6872da115a000000000025101080017085c2b78ca00004 08351000 200f


// take an IP address, and a MAC address to respond with and create search gateway response
func CreateSearchGatewayResponse(ipAddress string, macAddress []byte) []byte{
	// a valid message: []byte{0x12, 0x24, 0x0a, 0x0e, 0x31, 0x39, 0x32, 0x2e, 0x31, 0x36, 0x38, 0x2e, 0x31, 0x37, 0x38, 0x2e, 0x32, 0x31, 0x12, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0x88, 0xe9, 0xfe, 0x51, 0xc5, 0x46, 0x18, 0x01}
	version := uint32(1)
	uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	uuid = append(uuid, macAddress...)

	resp := proto.SearchGatewayResponse{
		Ipaddress:            &ipAddress,
		Uuid:                 uuid,
		Version:              &version,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}

	b, _ := resp.XXX_Marshal([]byte{0x12, 0x24}, false)
	return b
}
