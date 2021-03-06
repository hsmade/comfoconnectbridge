package comfoconnect

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/proto"
)

type OperationType interface { // FIXME: rename
	XXX_Unmarshal([]byte) error
	XXX_Marshal(b []byte, deterministic bool) ([]byte, error)
}

type Message struct {
	Src           []byte
	Dst           []byte
	Operation     proto.GatewayOperation
	RawMessage    []byte
	OperationType OperationType
	Span          opentracing.Span
}

func GetMessageFromSocket(conn net.Conn) (Message, error) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "GetMessageFromSocket",
		//"span": span.Context().(jaeger.SpanContext).String(),
	})

	var completeMessage []byte

	lengthBytes, err := ReadBytes(conn, 4)
	if err != nil {
		if opErr, ok := errors.Cause(err).(*net.OpError); ok && opErr.Timeout() {
			// read timeout, silently ignore
			//span.SetTag("err", err)
			return Message{}, err
		}
		err := errors.Wrap(err, "reading message length")
		log.Error(err)
		//span.SetTag("err", err)
		return Message{}, err
	}
	span := opentracing.StartSpan("comfoconnect.GetMessageFromSocket")
	defer span.Finish()
	span.SetTag("remote", conn.RemoteAddr().String())

	completeMessage = append(completeMessage, lengthBytes...)
	length := binary.BigEndian.Uint32(lengthBytes)

	if length < 0 || length > 1024 {
		err := errors.New(fmt.Sprintf("got invalid length: %d", length))
		log.Trace(err)
		span.SetTag("err", err)
		return Message{}, err
	}
	log.Trace("length: %d", length)

	src, err := ReadBytes(conn, 16)
	if err != nil {
		err := errors.Wrap(err, "reading Src")
		log.Error(err)
		span.SetTag("err", err)
		return Message{}, err
	}
	completeMessage = append(completeMessage, src...)
	log.Trace("Src: %x", src)

	dst, err := ReadBytes(conn, 16)
	if err != nil {
		err := errors.Wrap(err, "reading Dst")
		log.Error(err)
		span.SetTag("err", err)
		return Message{}, err
	}
	completeMessage = append(completeMessage, dst...)
	log.Trace("Dst: %x", dst)

	operationLengthBytes, err := ReadBytes(conn, 2)
	if err != nil {
		err := errors.Wrap(err, "reading operation length")
		log.Error(err)
		span.SetTag("err", err)
		return Message{}, err
	}
	completeMessage = append(completeMessage, operationLengthBytes...)
	operationLength := binary.BigEndian.Uint16(operationLengthBytes)

	if operationLength < 1 || operationLength > 1024 {
		err := errors.New(fmt.Sprintf("got invalid operationLength: %d", operationLength))
		log.Trace(err)
		span.SetTag("err", err)
		return Message{}, err
	}
	log.Trace("operationLength: %d", operationLength)

	operationBytes, err := ReadBytes(conn, int(operationLength))
	if err != nil {
		err := errors.Wrap(err, "reading operation")
		log.Error(err)
		span.SetTag("err", err)
		return Message{}, err
	}
	completeMessage = append(completeMessage, operationBytes...)
	log.Trace("operationBytes: %x", operationBytes)

	operationTypeLength := (length - 34) - uint32(operationLength)
	var operationTypeBytes []byte

	if operationTypeLength > 0 {
		log.Trace("operationTypeLength: %d", operationTypeLength)
		operationTypeBytes, err = ReadBytes(conn, int(operationTypeLength))
		if err != nil {
			err := errors.Wrap(err, "reading operation type")
			log.Error(err)
			span.SetTag("err", err)
			return Message{}, err
		}
		completeMessage = append(completeMessage, operationTypeBytes...)
		log.Trace("operationTypeBytes: %x", operationTypeBytes)
	}

	operation := proto.GatewayOperation{} // FIXME: parse instead of assume
	err = operation.XXX_Unmarshal(operationBytes)
	if err != nil {
		err := errors.Wrap(err, "failed to unmarshal operation")
		log.Error(err)
		span.SetTag("err", err)
		return Message{RawMessage: completeMessage}, err
	}

	operationType := GetStructForType(operation.Type.String())
	err = operationType.XXX_Unmarshal(operationTypeBytes)
	if err != nil {
		err := errors.Wrap(err, "failed to unmarshal operation type") // FIXME
		log.Error(err)
		span.SetTag("err", err)
		return Message{RawMessage: completeMessage}, err
	}

	message := Message{
		Src:           src,
		Dst:           dst,
		Operation:     operation,
		RawMessage:    completeMessage,
		OperationType: operationType,
		Span:          span,
	}

	//switch message.Operation.Type.String() {
	//case "CnRpdoNotificationType":
	//	actual := message.OperationType.(*proto.CnRpdoNotification)
	//	log.Debugf("Received Rpdo for ppid:%d with data:%x", *actual.Pdid, actual.Data)
	//case "CnRpdoRequestType":
	//	actual := message.OperationType.(*proto.CnRpdoRequest)
	//	log.Debugf("Received Rpdo request for ppid:%d", *actual.Pdid)
	//// TODO: decode RMI data ?
	//case "CnRmiRequestType":
	//	actual := message.OperationType.(*proto.CnRmiRequest)
	//	log.Debugf("Received Rmi request for node:%d with data:%x", *actual.NodeId, actual.Message)
	//case "CnRmiResponseType":
	//	actual := message.OperationType.(*proto.CnRmiResponse)
	//	log.Debugf("Received Rmi response with result:%d and data:%x", *actual.Result, actual.Message)
	//case "CnRmiAsyncRequestType":
	//	actual := message.OperationType.(*proto.CnRmiAsyncRequest)
	//	log.Debugf("Received Rmi async request for node:%d and data:%x", *actual.NodeId, actual.Message)
	//case "CnRmiAsyncResponseType":
	//	actual := message.OperationType.(*proto.CnRmiAsyncResponse)
	//	log.Debugf("Received Rmi async response with result:%d and data:%x", *actual.Result, actual.Message)
	//}
	SpanSetMessage(span, message)
	return message, nil
}

func (m Message) String() string {
	if m.Operation.Type == nil {
		return "Empty object"
	}
	var reference uint32
	if m.Operation.Reference != nil {
		reference = *m.Operation.Reference
	}
	result := fmt.Sprintf("Src=%x; Dst=%x; Cmd_type=%v; ref=%v; RawMessage=%x", m.Src, m.Dst, m.Operation.Type.String(), reference, m.RawMessage)

	//b, _ := json.Marshal(m)
	//result := string(b)
	return result
}

// creates the correct response message as a byte slice, for the parent message
func (m Message) CreateResponse(span opentracing.Span, status proto.GatewayOperation_GatewayResult) []byte {
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
	operation := proto.GatewayOperation{
		Type:      &responseType,
		Reference: message.Operation.Reference,
		Result:    &status,
	}
	if status == -1 {
		operation.Result = nil
	}

	responseStruct := GetStructForType(responseType.String())
	if responseStruct == nil {
		err := errors.New(fmt.Sprint("unable to find struct for type: %s", responseType.String()))
		log.Error(err)
		span.SetTag("err", err)
		return nil
	}

	//overrides
	switch responseType.String() {
	case "CnTimeConfirmType":
		currentTime := uint32(time.Now().Sub(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)).Seconds())
		responseStruct.(*proto.CnTimeConfirm).CurrentTime = &currentTime
	case "StartSessionConfirmType":
		ok := proto.GatewayOperation_OK
		operation.Result = &ok
	case "VersionConfirmType": // FIXME: get this from comfoconnect
		gw := uint32(1049610)
		cn := uint32(1073750016)
		serial := "DEM0116371204"
		responseStruct.(*proto.VersionConfirm).GatewayVersion = &gw
		responseStruct.(*proto.VersionConfirm).ComfoNetVersion = &cn
		responseStruct.(*proto.VersionConfirm).SerialNumber = &serial
		ok := proto.GatewayOperation_OK
		operation.Result = &ok
	case "GetRemoteAccessIdConfirmType": // FIXME: get this from comfoconnect
		uuid := "7m\351\332}\322C\346\270\336^G\307\223Y\\"
		responseStruct.(*proto.GetRemoteAccessIdConfirm).Uuid = []byte(uuid)
	case "CnRmiResponseType":
		request := m.OperationType.(*proto.CnRmiRequest).Message
		log.Debugf("Responding to CnRmiRequest(%x)", request)
		// first request TODO: replace with actual call to comfoconnect
		if bytes.Compare(request, []byte{0x02, 0x01, 0x01, 0x01, 0x15, 0x03, 0x04, 0x06, 0x05, 0x07}) == 0 {
			responseData := []byte{0x02, 0x42, 0x45, 0x41, 0x30, 0x30, 0x34, 0x31, 0x38, 0x35, 0x30, 0x33, 0x31, 0x39, 0x31, 0x30, 0x00, 0x00, 0x10, 0x10, 0xc0, 0x02, 0x00, 0x54, 0x10, 0x40}
			responseStruct.(*proto.CnRmiResponse).Message = responseData
		}

		// second request TODO: replace with actual call to comfoconnect
		if bytes.Compare(request, []byte{0x87, 0x15, 0x01}) == 0 {
			responseData := []byte{0x0b, 0x01, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x1c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x1c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x58, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x58, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x58, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0xb0, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			responseStruct.(*proto.CnRmiResponse).Message = responseData
		}

		// second request TODO: replace with actual call to comfoconnect
		if bytes.Compare(request, []byte{0x87, 0x15, 0x05}) == 0 {
			responseData := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
			responseStruct.(*proto.CnRmiResponse).Message = responseData
		}

	}
	result := message.packMessage(operation, responseStruct)
	span.SetTag("result", result)
	return result
}

func (m Message) CreateCustomResponse(span opentracing.Span, operationType proto.GatewayOperation_OperationType, operationTypeStruct OperationType) []byte {
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
	operation := proto.GatewayOperation{
		Type:      &operationType,
		Reference: m.Operation.Reference, // if we add this, we get double reference (prefixed)??
		Result:    nil,
	}

	return m.packMessage(operation, operationTypeStruct)
}

// setup a binary message ready to send
func (m Message) packMessage(operation proto.GatewayOperation, operationType OperationType) []byte {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Message",
		"method": "packMessage",
	})

	operationBytes, _ := operation.XXX_Marshal(nil, false)
	log.Trace("operationBytes: %x", operationBytes)
	operationTypeBytes, _ := operationType.XXX_Marshal(nil, false)
	log.Trace("operationTypeBytes: %x", operationTypeBytes)
	response := make([]byte, 4)
	binary.BigEndian.PutUint32(response, uint32(len(operationTypeBytes)+34+len(operationBytes))) // raw message length
	log.Trace("length: %x", response)
	response = append(response, m.Src...)
	response = append(response, m.Dst...)
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(len(operationBytes))) // op length
	log.Trace("op length: %x", b)
	response = append(response, b...)
	response = append(response, operationBytes...) // gatewayOperation
	response = append(response, operationTypeBytes...)

	return response
}

func (m Message) Encode() []byte {
	return m.packMessage(m.Operation, m.OperationType)
}

func (m Message) DecodePDO() RpdoTypeConverter {
	if m.Operation.Type.String() != "CnRpdoNotificationType" {
		return nil
	}
	ppid := m.OperationType.(*proto.CnRpdoNotification).Pdid
	data := m.OperationType.(*proto.CnRpdoNotification).Data
	return NewPpid(*ppid, data)
}

// take an IP address, and a MAC address to respond with and create search gateway response
func CreateSearchGatewayResponse(ipAddress string, uuid []byte) []byte {
	version := uint32(1)
	//uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	//uuid = append(uuid, macAddress...)

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

// takes the name for an operation type and finds the struct for it
func GetStructForType(operationTypeString string) OperationType {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "GetStructForType",
	})

	var operationType OperationType
	switch operationTypeString {
	case "SetAddressRequestType":
		operationType = &proto.SetAddressRequest{}
	case "RegisterAppRequestType":
		operationType = &proto.RegisterAppRequest{}
	case "StartSessionRequestType":
		operationType = &proto.StartSessionRequest{}
	case "CloseSessionRequestType":
		operationType = &proto.CloseSessionRequest{}
	case "ListRegisteredAppsRequestType":
		operationType = &proto.ListRegisteredAppsRequest{}
	case "DeregisterAppRequestType":
		operationType = &proto.DeregisterAppRequest{}
	case "ChangePinRequestType":
		operationType = &proto.ChangePinRequest{}
	case "GetRemoteAccessIdRequestType":
		operationType = &proto.GetRemoteAccessIdRequest{}
	case "SetRemoteAccessIdRequestType":
		operationType = &proto.SetRemoteAccessIdRequest{}
	case "GetSupportIdRequestType":
		operationType = &proto.GetSupportIdRequest{}
	case "SetSupportIdRequestType":
		operationType = &proto.SetSupportIdRequest{}
	case "GetWebIdRequestType":
		operationType = &proto.GetWebIdRequest{}
	case "SetWebIdRequestType":
		operationType = &proto.SetWebIdRequest{}
	case "SetPushIdRequestType":
		operationType = &proto.SetPushIdRequest{}
	case "DebugRequestType":
		operationType = &proto.DebugRequest{}
	case "UpgradeRequestType":
		operationType = &proto.UpgradeRequest{}
	case "SetDeviceSettingsRequestType":
		operationType = &proto.SetDeviceSettingsRequest{}
	case "VersionRequestType":
		operationType = &proto.VersionRequest{}
	case "SetAddressConfirmType":
		operationType = &proto.SetAddressConfirm{}
	case "RegisterAppConfirmType":
		operationType = &proto.RegisterAppConfirm{}
	case "StartSessionConfirmType":
		operationType = &proto.StartSessionConfirm{}
	case "CloseSessionConfirmType":
		operationType = &proto.CloseSessionConfirm{}
	case "ListRegisteredAppsConfirmType":
		operationType = &proto.ListRegisteredAppsConfirm{}
	case "DeregisterAppConfirmType":
		operationType = &proto.DeregisterAppConfirm{}
	case "ChangePinConfirmType":
		operationType = &proto.ChangePinConfirm{}
	case "GetRemoteAccessIdConfirmType":
		operationType = &proto.GetRemoteAccessIdConfirm{}
	case "SetRemoteAccessIdConfirmType":
		operationType = &proto.SetRemoteAccessIdConfirm{}
	case "GetSupportIdConfirmType":
		operationType = &proto.GetSupportIdConfirm{}
	case "SetSupportIdConfirmType":
		operationType = &proto.SetSupportIdConfirm{}
	case "GetWebIdConfirmType":
		operationType = &proto.GetWebIdConfirm{}
	case "SetWebIdConfirmType":
		operationType = &proto.SetWebIdConfirm{}
	case "SetPushIdConfirmType":
		operationType = &proto.SetPushIdConfirm{}
	case "DebugConfirmType":
		operationType = &proto.DebugConfirm{}
	case "UpgradeConfirmType":
		operationType = &proto.UpgradeConfirm{}
	case "SetDeviceSettingsConfirmType":
		operationType = &proto.SetDeviceSettingsConfirm{}
	case "VersionConfirmType":
		operationType = &proto.VersionConfirm{}
	case "GatewayNotificationType":
		operationType = &proto.GatewayNotification{}
	case "KeepAliveType":
		operationType = &proto.KeepAlive{}
	case "FactoryResetType":
		operationType = &proto.FactoryReset{}
	case "CnTimeRequestType":
		operationType = &proto.CnTimeRequest{}
	case "CnTimeConfirmType":
		operationType = &proto.CnTimeConfirm{}
	case "CnNodeRequestType":
		operationType = &proto.CnNodeRequest{}
	case "CnNodeNotificationType":
		operationType = &proto.CnNodeNotification{}
	case "CnRmiRequestType":
		operationType = &proto.CnRmiRequest{}
	case "CnRmiResponseType":
		operationType = &proto.CnRmiResponse{}
	case "CnRmiAsyncRequestType":
		operationType = &proto.CnRmiAsyncRequest{}
	case "CnRmiAsyncConfirmType":
		operationType = &proto.CnRmiAsyncConfirm{}
	case "CnRmiAsyncResponseType":
		operationType = &proto.CnRmiAsyncResponse{}
	case "CnRpdoRequestType":
		operationType = &proto.CnRpdoRequest{}
	case "CnRpdoConfirmType":
		operationType = &proto.CnRpdoConfirm{}
	case "CnRpdoNotificationType":
		operationType = &proto.CnRpdoNotification{}
	case "CnAlarmNotificationType":
		operationType = &proto.CnAlarmNotification{}
	case "CnFupReadRegisterRequestType":
		operationType = &proto.CnFupReadRegisterRequest{}
	case "CnFupReadRegisterConfirmType":
		operationType = &proto.CnFupReadRegisterConfirm{}
	case "CnFupProgramBeginRequestType":
		operationType = &proto.CnFupProgramBeginRequest{}
	case "CnFupProgramBeginConfirmType":
		operationType = &proto.CnFupProgramBeginConfirm{}
	case "CnFupProgramRequestType":
		operationType = &proto.CnFupProgramRequest{}
	case "CnFupProgramConfirmType":
		operationType = &proto.CnFupProgramConfirm{}
	case "CnFupProgramEndRequestType":
		operationType = &proto.CnFupProgramEndRequest{}
	case "CnFupProgramEndConfirmType":
		operationType = &proto.CnFupProgramEndConfirm{}
	case "CnFupReadRequestType":
		operationType = &proto.CnFupReadRequest{}
	case "CnFupReadConfirmType":
		operationType = &proto.CnFupReadConfirm{}
	case "CnFupResetRequestType":
		operationType = &proto.CnFupResetRequest{}
	case "CnFupResetConfirmType":
		operationType = &proto.CnFupResetConfirm{}
	default:
		operationType = nil
	}

	if operationType == nil {
		log.Errorf("unable to find matching struct for operation type: %s", operationTypeString)
	} else {
		log.Debugf("found struct: %s, for operation type:%s", reflect.TypeOf(operationType).Elem().Name(), operationTypeString)
	}
	return operationType
}

// takes an operation type and finds the correct response type
func getResponseTypeForOperationType(operationType OperationType) proto.GatewayOperation_OperationType {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "getResponseTypeForOperationType",
	})

	var responseTypeString proto.GatewayOperation_OperationType
	operationTypeString := reflect.TypeOf(operationType).Elem().Name()

	switch operationTypeString {
	case "SetAddressRequest":
		responseTypeString = proto.GatewayOperation_SetAddressConfirmType
	case "RegisterAppRequest":
		responseTypeString = proto.GatewayOperation_RegisterAppConfirmType
	case "StartSessionRequest":
		responseTypeString = proto.GatewayOperation_StartSessionConfirmType
	case "CloseSessionRequest":
		responseTypeString = proto.GatewayOperation_CloseSessionConfirmType
	case "ListRegisteredAppsRequest":
		responseTypeString = proto.GatewayOperation_ListRegisteredAppsConfirmType
	case "DeregisterAppRequest":
		responseTypeString = proto.GatewayOperation_SetAddressConfirmType
	case "ChangePinRequest":
		responseTypeString = proto.GatewayOperation_ChangePinConfirmType
	case "GetRemoteAccessIdRequest":
		responseTypeString = proto.GatewayOperation_GetRemoteAccessIdConfirmType
	case "SetRemoteAccessIdRequest":
		responseTypeString = proto.GatewayOperation_SetRemoteAccessIdConfirmType
	case "GetSupportIdRequest":
		responseTypeString = proto.GatewayOperation_GetSupportIdConfirmType
	case "GetWebIdRequest":
		responseTypeString = proto.GatewayOperation_GetWebIdConfirmType
	case "SetWebIdRequest":
		responseTypeString = proto.GatewayOperation_SetWebIdConfirmType
	case "SetPushIdRequest":
		responseTypeString = proto.GatewayOperation_SetPushIdConfirmType
	case "DebugRequest":
		responseTypeString = proto.GatewayOperation_DebugConfirmType
	case "UpgradeRequest":
		responseTypeString = proto.GatewayOperation_UpgradeConfirmType
	case "SetDeviceSettingsRequest":
		responseTypeString = proto.GatewayOperation_SetDeviceSettingsConfirmType
	case "VersionRequest":
		responseTypeString = proto.GatewayOperation_VersionConfirmType
	case "CnTimeRequest":
		responseTypeString = proto.GatewayOperation_CnTimeConfirmType
	case "CnRmiRequest":
		responseTypeString = proto.GatewayOperation_CnRmiResponseType
	case "CnRmiAsyncRequest":
		responseTypeString = proto.GatewayOperation_CnRmiAsyncConfirmType
	case "CnRpdoRequest":
		responseTypeString = proto.GatewayOperation_CnRpdoConfirmType
	case "CnFupReadRegisterRequest":
		responseTypeString = proto.GatewayOperation_CnFupReadRegisterConfirmType
	case "CnFupProgramBeginRequest":
		responseTypeString = proto.GatewayOperation_CnFupProgramBeginConfirmType
	case "CnFupProgramRequest":
		responseTypeString = proto.GatewayOperation_CnFupProgramConfirmType
	case "CnFupProgramEndRequest":
		responseTypeString = proto.GatewayOperation_CnFupProgramEndConfirmType
	case "CnFupReadRequest":
		responseTypeString = proto.GatewayOperation_CnFupReadConfirmType
	case "CnFupResetRequest":
		responseTypeString = proto.GatewayOperation_CnFupResetConfirmType
	}

	if responseTypeString == 0 {
		log.Errorf("unable to find response type for operation type: %s", operationTypeString)
	} else {
		log.Debugf("found response type: %s for operation type: %s", responseTypeString, operationTypeString)
	}
	return responseTypeString
}

func ReadBytes(conn net.Conn, size int) ([]byte, error) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "ReadBytes",
		"size":   size,
	})
	log.Debugf("reading from %s", conn.RemoteAddr().String())

	if size < 1 {
		err := errors.New(fmt.Sprintf("Invalid size: %d", size))
		log.Error(err)
		return nil, err
	}
	var result []byte
	for {
		//err := conn.SetReadDeadline(time.Now().Add(time.Second * 1))
		//if err != nil {
		//	return nil, errors.Wrap(err, "setting timeout for read")
		//}

		buffer := make([]byte, size)
		readLen, err := conn.Read(buffer)
		if err != nil {
			return nil, errors.Wrap(err, "reading from socket")
		}

		if readLen > 0 {
			size -= readLen
			result = append(result, buffer[:readLen]...)
			log.Trace("read result now: %x, read bytes:%d", result, readLen)
		}

		if size == 0 {
			break
		}

		if size < 0 {
			return nil, errors.New("read too many bytes: size")
		}
	}
	return result, nil
}

func SpanSetMessage(span opentracing.Span, message Message) {
	span.SetTag("messsage", message)
	span.SetTag("src", fmt.Sprintf("%x", message.Src))
	span.SetTag("dst", fmt.Sprintf("%x", message.Dst))
	reference := message.Operation.Reference
	if reference != nil {
		span.SetTag("reference", fmt.Sprintf("%d", *reference))
	} else {
		span.SetTag("reference", "nil")
	}
	span.SetTag("operationType", reflect.TypeOf(message.OperationType))
}
