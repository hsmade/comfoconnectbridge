package comfoconnect

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/hsmade/comfoconnectbridge/pb"

	"github.com/golang/protobuf/proto"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "NewMessageFromSocket",
		//"span": span.Context().(jaeger.SpanContext).String(),
	})
	log.Debug("Reading message from socket")

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
		err := errors.Wrap(err, "reading message length from socket")
		log.Error(err)
		return nil, err
	}
	if length > 1024 {
		err := errors.New(fmt.Sprintf("Invalid length received for message: %d", length))
		log.Error(err)
		return nil, err
	}
	log.Tracef("message length: %d", length)

	message.Src, err = ReadBytes(conn, len(message.Src))
	if err != nil {
		err := errors.Wrap(err, "reading src address from socket")
		log.Error(err)
		return nil, err
	}
	log.Tracef("src: %x", message.Src)

	message.Dst, err = ReadBytes(conn, len(message.Dst))
	if err != nil {
		err := errors.Wrap(err, "reading dst address from socket")
		log.Error(err)
		return nil, err
	}
	log.Tracef("dst: %x", message.Dst)

	var operationLength uint16
	err = binary.Read(conn, binary.BigEndian, &operationLength)
	if err != nil {
		err := errors.Wrap(err, "reading operation length from socket")
		log.Error(err)
		return nil, err
	}
	if operationLength > 1024 {
		err := errors.New(fmt.Sprintf("Invalid length received for operation: %d", operationLength))
		log.Error(err)
		return nil, err
	}
	log.Tracef("operation length: %d", operationLength)

	operationBytes := make([]byte, operationLength)
	operationBytes, err = ReadBytes(conn, int(operationLength))
	if err != nil {
		err := errors.Wrap(err, "reading operation from socket")
		log.Error(err)
		return nil, err
	}
	log.Tracef("operation bytes: %x", operationBytes)

	// There is only one operation, even in the android code
	err = proto.Unmarshal(operationBytes, message.Operation)
	if err != nil {
		err := errors.Wrap(err, "parsing operation bytes into operation struct")
		log.Error(err)
		return nil, err
	}
	log.Tracef("operation type: %s", message.Operation.Type.String())

	operationTypeLength := (length - 34) - uint32(operationLength)
	OperationTypeBytes := make([]byte, operationTypeLength)
	log.Tracef("operation type length: %d", operationTypeLength)
	if operationTypeLength > 0 {
		OperationTypeBytes, err = ReadBytes(conn, int(operationTypeLength))
		if err != nil {
			err := errors.Wrap(err, "reading operation type from socket")
			log.Error(err)
			return nil, err
		}
		log.Tracef("operation type bytes: %x", OperationTypeBytes)
	}

	// assign operation type to empty struct of right type
	message.OperationType = GetStructForType(message.Operation.Type.String())
	err = proto.Unmarshal(OperationTypeBytes, message.OperationType)
	if err != nil {
		err := errors.Wrap(err, "parsing operation type bytes into operation struct")
		log.Error(err)
		return nil, err
	}

	log.Debugf("read message; %v", message)
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
// FIXME: return new message, rename the func
func (m Message) CreateResponse(span opentracing.Span, status pb.GatewayOperation_GatewayResult) []byte {
	// FIXME: span should not be here?
	if span == nil {
		span = opentracing.StartSpan("comfoconnect.Message.CreateResponse")
	} else {
		span = opentracing.GlobalTracer().StartSpan("comfoconnect.Message.CreateResponse", opentracing.ChildOf(span.Context()))
	}
	defer span.Finish()
	span.SetTag("status", status)

	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Message",
		"method": "CreateResponse",
		//"span": span.Context().(jaeger.SpanContext).String(),
	})

	message := m
	message.Src = m.Dst
	message.Dst = m.Src
	log.Debugf("creating response for operation type: %s", reflect.TypeOf(m.OperationType).Elem().Name())
	responseType := getResponseTypeForOperationType(message.OperationType)
	span.SetTag("responseType", responseType.String())
	operation := pb.GatewayOperation{
		Type:      &responseType,
		Reference: message.Operation.Reference,
		Result:    &status,
	}
	if status == -1 {
		operation.Result = nil
	}

	responseStruct := GetStructForType(responseType.String())
	if responseStruct == nil {
		err := errors.New(fmt.Sprintf("unable to find struct for type: %s", responseType.String()))
		log.Error(err)
		span.SetTag("err", err)
		return nil
	}

	//overrides
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
		log.Debugf("Responding to CnRmiRequest(%x)", request)
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
	result := message.packMessage(&operation, responseStruct)
	span.SetTag("result", result)
	return result
}

func (m Message) CreateCustomResponse(span opentracing.Span, operationType pb.GatewayOperation_OperationType, operationTypeStruct OperationType) []byte {
	if span == nil {
		span = opentracing.StartSpan("comfoconnect.Message.CreateCustomResponse")
	} else {
		span = opentracing.GlobalTracer().StartSpan("comfoconnect.Message.CreateCustomResponse", opentracing.ChildOf(span.Context()))
	}
	defer span.Finish()
	span.SetTag("operationType", operationType.String())

	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Message",
		"method": "CreateCustomResponse",
		//"span": span.Context().(jaeger.SpanContext).String(),
	})

	log.Debugf("creating custom response for operation type: %s", reflect.TypeOf(operationTypeStruct).Elem().Name())
	operation := pb.GatewayOperation{
		Type:      &operationType,
		Reference: m.Operation.Reference, // if we add this, we get double reference (prefixed)??
		Result:    nil,
	}

	return m.packMessage(&operation, operationTypeStruct)
}

// setup a binary message ready to send
func (m Message) packMessage(operation *pb.GatewayOperation, operationType OperationType) []byte {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Message",
		"method": "packMessage",
	})

	operationBytes, _ := proto.Marshal(operation)
	log.Tracef("operationBytes: %x", operationBytes)
	operationTypeBytes, _ := proto.Marshal(operationType)
	log.Tracef("operationTypeBytes: %x", operationTypeBytes)
	response := make([]byte, 4)
	binary.BigEndian.PutUint32(response, uint32(len(operationTypeBytes)+34+len(operationBytes))) // raw message length
	log.Tracef("length: %x", response)
	response = append(response, m.Src...)
	response = append(response, m.Dst...)
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(len(operationBytes))) // op length
	log.Tracef("op length: %x", b)
	response = append(response, b...)
	response = append(response, operationBytes...) // gatewayOperation
	response = append(response, operationTypeBytes...)

	return response
}

// FIXME: remove this
func (m Message) Encode() []byte {
	return m.packMessage(m.Operation, m.OperationType)
}

// FIXME: remove this
func (m Message) DecodePDO() RpdoTypeConverter {
	if m.Operation.Type.String() != "CnRpdoNotificationType" {
		return nil
	}
	ppid := m.OperationType.(*pb.CnRpdoNotification).Pdid
	data := m.OperationType.(*pb.CnRpdoNotification).Data
	return NewPpid(*ppid, data)
}
