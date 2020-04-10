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
	return fmt.Sprintf("src=%x; dst=%x; Cmd_type=%v; ref=%v; msg=%x", m.src, m.dst, m.Cmd.Type.String(), *m.Cmd.Reference, m.msg)
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
	response := make([]byte, 4)
	binary.BigEndian.PutUint32(response, uint32(len(msg) + 34 + len(cmdBytes))) // msg length
	logrus.Debugf("size message(%d): %x", len(msg), response)
	response = append(response, m.dst...)
	logrus.Debugf("src message: %x", response)
	response = append(response, m.src...)
	logrus.Debugf("dst message: %x", response)
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(len(cmdBytes))) // cmd length
	response = append(response, b...)
	logrus.Debugf("len message: %x", response)
	response = append(response, cmdBytes...) // gatewayOperation
	logrus.Debugf("cmd message: %x", response)
	response = append(response, msg...)
	logrus.Debugf("msg message: %x", response)

	return response
}

// 00000026 0000000000251010800170b3d54264b4 af154804169043898d2da77148f886be 0004 083420 02 RegisterAppConfirm example
// 00000026 000000000025101080017085c2b78ca0 eda4982303bb4ca49ddefc434f64bd8d 0004 083420 00 RegisterAppConfirm valid, fixed src/dst

// 00000028 0000000000251010800170b3d54264b4 af154804169043898d2da77148f886be 0006 083510 0020 03 StartSessionConfirm example valid
// 00000028 000000000025101080017085c2b78ca0 eda4982303bb4ca49ddefc434f64bd8d 0006 083510 0020 02 StartSessionConfirm, src/dst, invalid

// take an IP address, and a MAC address to respond with and create search gateway response
func CreateSearchGatewayResponse(ipAddress string, macAddress []byte) []byte{
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
