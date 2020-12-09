package comfoconnect

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/hsmade/comfoconnectbridge/pb"
	"github.com/hsmade/comfoconnectbridge/pkg/helpers"

	"github.com/golang/protobuf/proto"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type OperationType proto.Message

type Message struct {
	Src           []byte
	Dst           []byte
	Operation     *pb.GatewayOperation
	RawMessage    []byte
	OperationType OperationType
	Span          opentracing.Span
}

// try to read a single message from a network socket
func NewMessageFromSocket(conn net.Conn) (*Message, error) {
	span := opentracing.StartSpan("comfoconnect.GetMessageFromSocket2")
	helpers.StackLogger().Debug("Reading message from socket")

	var err error
	message := Message{
		Span:      span,
		Src:       make([]byte, 16),
		Dst:       make([]byte, 16),
		Operation: &pb.GatewayOperation{},
	}

	var length uint32
	err = binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			return nil, err // don't want to log timeout errors
		}
		return nil, helpers.LogOnError(errors.Wrap(err, "reading message length from socket"))
	}
	if length > 1024 {
		return nil, helpers.LogOnError(errors.New(fmt.Sprintf("Invalid length received for message: %d", length)))
	}
	helpers.StackLogger().Tracef("message length: %d", length)

	message.Src, err = ReadBytes(conn, len(message.Src))
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "reading src address from socket"))
	}
	helpers.StackLogger().Tracef("src: %x", message.Src)

	message.Dst, err = ReadBytes(conn, len(message.Dst))
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "reading dst address from socket"))
	}
	helpers.StackLogger().Tracef("dst: %x", message.Dst)

	var operationLength uint16
	err = binary.Read(conn, binary.BigEndian, &operationLength)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "reading operation length from socket"))
	}
	if operationLength > 1024 {
		return nil, helpers.LogOnError(errors.New(fmt.Sprintf("Invalid length received for operation: %d", operationLength)))
	}
	helpers.StackLogger().Tracef("operation length: %d", operationLength)

	operationBytes := make([]byte, operationLength)
	operationBytes, err = ReadBytes(conn, int(operationLength))
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "reading operation from socket"))
	}
	helpers.StackLogger().Tracef("operation bytes: %x", operationBytes)

	// There is only one operation, even in the android code
	err = proto.Unmarshal(operationBytes, message.Operation)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "parsing operation bytes into operation struct"))
	}
	helpers.StackLogger().Tracef("operation type: %s", message.Operation.Type.String())

	operationTypeLength := (length - 34) - uint32(operationLength)
	OperationTypeBytes := make([]byte, operationTypeLength)
	helpers.StackLogger().Tracef("operation type length: %d", operationTypeLength)
	if operationTypeLength > 0 {
		OperationTypeBytes, err = ReadBytes(conn, int(operationTypeLength))
		if err != nil {
			return nil, helpers.LogOnError(errors.Wrap(err, "reading operation type from socket"))
		}
		helpers.StackLogger().Tracef("operation type bytes: %x", OperationTypeBytes)
	}

	// assign operation type to empty struct of right type
	message.OperationType = GetStructForType(message.Operation.Type.String())
	err = proto.Unmarshal(OperationTypeBytes, message.OperationType)
	if err != nil {
		return nil, helpers.LogOnError(errors.Wrap(err, "parsing operation type bytes into operation struct"))
	}

	helpers.StackLogger().Debugf("read message; %v", message)
	span.SetTag("message", message)
	return &message, nil
}

// displays all the important information about a message
func (m Message) String() string {
	if m.Operation.Type == nil {
		return "Empty object"
	}
	var reference uint32
	if m.Operation.Reference != nil {
		reference = *m.Operation.Reference
	}
	result := fmt.Sprintf("Src=%x; Dst=%x; Opr_type=%v; ref=%v; Opr:=%v", m.Src, m.Dst, m.Operation.Type.String(), reference, m.OperationType)
	return result
}

// creates the correct response message as a byte slice, for the parent message
func (m Message) CreateResponse(status pb.GatewayOperation_GatewayResult) (*Message, error) {
	helpers.StackLogger().Debugf("creating response for operation type: %s", reflect.TypeOf(m.OperationType).Elem().Name())
	message := Message{
		// copy these, swapped from the original message
		Src: m.Dst,
		Dst: m.Src,
	}
	responseType := getResponseTypeForOperationType(m.OperationType)
	operation := pb.GatewayOperation{
		Type:      &responseType,
		Reference: m.Operation.Reference,
		Result:    &status,
	}
	message.Operation = &operation

	if status == -1 {
		operation.Result = nil
	}

	responseStruct := GetStructForType(responseType.String())
	if responseStruct == nil {
		return nil, helpers.LogOnError(errors.New(fmt.Sprintf("unable to find struct for type: %s", responseType.String())))
	}

	// set the data for the operation type
	// FIXME: use the builder pattern?
	switch responseType.String() {
	case "CnTimeConfirmType":
		currentTime := uint32(time.Now().Sub(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)).Seconds())
		responseStruct.(*pb.CnTimeConfirm).CurrentTime = &currentTime
	case "StartSessionConfirmType":
		ok := pb.GatewayOperation_OK
		operation.Result = &ok
	case "VersionConfirmType": // FIXME: get this from comfoconnect
		gw := uint32(1049610)
		cn := uint32(1073750016)
		serial := "DEM0116371204"
		responseStruct.(*pb.VersionConfirm).GatewayVersion = &gw
		responseStruct.(*pb.VersionConfirm).ComfoNetVersion = &cn
		responseStruct.(*pb.VersionConfirm).SerialNumber = &serial
		ok := pb.GatewayOperation_OK
		operation.Result = &ok
	case "GetRemoteAccessIdConfirmType": // FIXME: get this from comfoconnect
		uuid := "7m\351\332}\322C\346\270\336^G\307\223Y\\"
		responseStruct.(*pb.GetRemoteAccessIdConfirm).Uuid = []byte(uuid)
	case "CnRmiResponseType":
		request := m.OperationType.(*pb.CnRmiRequest).Message
		helpers.StackLogger().Debugf("Responding to CnRmiRequest(%x)", request)
		// first request TODO: replace with actual call to comfoconnect
		if bytes.Compare(request, []byte{0x02, 0x01, 0x01, 0x01, 0x15, 0x03, 0x04, 0x06, 0x05, 0x07}) == 0 {
			responseData := []byte{0x02, 0x42, 0x45, 0x41, 0x30, 0x30, 0x34, 0x31, 0x38, 0x35, 0x30, 0x33, 0x31, 0x39, 0x31, 0x30, 0x00, 0x00, 0x10, 0x10, 0xc0, 0x02, 0x00, 0x54, 0x10, 0x40}
			responseStruct.(*pb.CnRmiResponse).Message = responseData
		}

		// second request TODO: replace with actual call to comfoconnect
		if bytes.Compare(request, []byte{0x87, 0x15, 0x01}) == 0 {
			responseData := []byte{0x0b, 0x01, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x1c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x1c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x58, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x58, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x58, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0xb0, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			responseStruct.(*pb.CnRmiResponse).Message = responseData
		}

		// second request TODO: replace with actual call to comfoconnect
		if bytes.Compare(request, []byte{0x87, 0x15, 0x05}) == 0 {
			responseData := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
			responseStruct.(*pb.CnRmiResponse).Message = responseData
		}

	}
	message.Operation = &operation
	message.OperationType = responseStruct
	return &message, nil
}

func (m Message) CreateCustomResponse(operationType pb.GatewayOperation_OperationType, operationTypeStruct OperationType) (*Message, error) {
	helpers.StackLogger().Debugf("creating custom response for operation type: %s", reflect.TypeOf(operationTypeStruct).Elem().Name())
	operation := pb.GatewayOperation{
		Type:      &operationType,
		Reference: m.Operation.Reference,
		Result:    nil,
	}

	message := Message{
		Src:           m.Dst,
		Dst:           m.Src,
		Operation:     &operation,
		OperationType: operationTypeStruct,
	}
	return &message, nil
}

// Encode will Marshall the contents of a message into bytes
func (m Message) Encode() []byte {
	operationBytes, _ := proto.Marshal(m.Operation)
	helpers.StackLogger().Tracef("operationBytes: %x", operationBytes)

	operationTypeBytes, _ := proto.Marshal(m.OperationType)
	helpers.StackLogger().Tracef("operationTypeBytes: %x", operationTypeBytes)

	response := make([]byte, 4)
	binary.BigEndian.PutUint32(response, uint32(len(operationTypeBytes)+34+len(operationBytes))) // raw message length
	helpers.StackLogger().Tracef("length: %x", response)

	response = append(response, m.Src...)
	response = append(response, m.Dst...)

	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(len(operationBytes))) // op length
	helpers.StackLogger().Tracef("op length: %x", b)
	response = append(response, b...)

	response = append(response, operationBytes...) // gatewayOperation
	response = append(response, operationTypeBytes...)

	return response
}

// FIXME: remove this?
func (m Message) DecodePDO() RpdoTypeConverter {
	if m.Operation.Type.String() != "CnRpdoNotificationType" {
		return nil
	}
	ppid := m.OperationType.(*pb.CnRpdoNotification).Pdid
	data := m.OperationType.(*pb.CnRpdoNotification).Data
	return NewPpid(*ppid, data)
}

func (m *Message) Send(conn net.Conn) error {
	helpers.StackLogger().Debugf("Sending '%v' to %s", *m, conn.RemoteAddr().String())
	data := m.Encode()
	_, err := conn.Write(data)
	return helpers.LogOnError(err)
}
