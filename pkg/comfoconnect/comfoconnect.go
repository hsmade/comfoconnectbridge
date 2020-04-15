package comfoconnect

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"

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
}

func GetMessageFromSocket(conn net.Conn) (Message, error) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "GetMessageFromSocket",
	})

	var completeMessage []byte

	lengthBytes, err := readBytes(conn, 4)
	if err != nil {
		log.Errorf("failed to read message length int: %v", err)
		return Message{}, errors.Wrap(err, "reading message length")
	}
	completeMessage = append(completeMessage, lengthBytes...)
	length := binary.BigEndian.Uint32(lengthBytes)

	if length < 0 || length > 1024 {
		msg := fmt.Sprintf("got invalid length: %d", length)
		log.Debug(msg)
		return Message{}, errors.New(msg)
	}
	log.Debugf("length: %d", length)

	src, err := readBytes(conn, 16)
	if err != nil {
		log.Errorf("failed to read Src: %v", err)
		return Message{}, errors.Wrap(err, "reading Src")
	}
	completeMessage = append(completeMessage, src...)
	log.Debugf("Src: %x", src)

	dst, err := readBytes(conn, 16)
	if err != nil {
		log.Errorf("failed to read Dst: %v", err)
		return Message{}, errors.Wrap(err, "reading Dst")
	}
	completeMessage = append(completeMessage, dst...)
	log.Debugf("Dst: %x", dst)

	operationLengthBytes, err := readBytes(conn, 2)
	if err != nil {
		log.Errorf("failed to read operation length int: %v", err)
		return Message{}, errors.Wrap(err, "reading operation length")
	}
	completeMessage = append(completeMessage, operationLengthBytes...)
	operationLength := binary.BigEndian.Uint16(operationLengthBytes)
	operationLength = 4 // FIXME: sign error above?

	if operationLength < 1 || operationLength > 1024 {
		msg := fmt.Sprintf("got invalid operationLength: %d", operationLength)
		log.Debug(msg)
		return Message{}, errors.New(msg)
	}
	log.Debugf("operationLength: %d", operationLength)

	operationBytes, err := readBytes(conn, int(operationLength))
	if err != nil {
		log.Errorf("failed to read operation: %v", err)
		return Message{}, errors.Wrap(err, "reading operation")
	}
	completeMessage = append(completeMessage, operationBytes...)
	log.Debugf("operationBytes: %x", operationBytes)

	operationTypeLength := (length - 34) - uint32(operationLength)
	var operationTypeBytes []byte

	if operationTypeLength > 0 {
		log.Debugf("operationTypeLength: %d", operationTypeLength)
		operationTypeBytes, err = readBytes(conn, int(operationTypeLength))
		if err != nil {
			log.Errorf("failed to read operationTypeBytes: %v", err)
			return Message{}, errors.Wrap(err, "reading operation type")
		}
		completeMessage = append(completeMessage, operationTypeBytes...)
		log.Debugf("operationTypeBytes: %x", operationTypeBytes)
	}

	operation := proto.GatewayOperation{} // FIXME: parse instead of assume
	err = operation.XXX_Unmarshal(operationBytes)
	if err != nil {
		return Message{}, errors.Wrap(err, "failed to unmarshal operation")
	}

	operationType := getStructForType(operation.Type.String())
	err = operationType.XXX_Unmarshal(operationTypeBytes)
	if err != nil {
		log.Errorf("failed to unmarshal operation type for operation=%v and bytes:%x coming from src=%x and remote=%s", operation.Type.String(), operationTypeBytes, src, conn.RemoteAddr().String())
		return Message{}, errors.Wrap(err, "failed to unmarshal operation type") // FIXME
	}

	message := Message{
		Src:           src,
		Dst:           dst,
		Operation:     operation,
		RawMessage:    completeMessage,
		OperationType: operationType,
	}

	switch message.Operation.Type.String() {
	case "CnRpdoNotificationType":
		actual := message.OperationType.(*proto.CnRpdoNotification)
		log.Debugf("Received Rpdo for ppid:%d with data:%x", *actual.Pdid, actual.Data)
	case "CnRpdoRequestType":
		actual := message.OperationType.(*proto.CnRpdoRequest)
		log.Debugf("Received Rpdo request for ppid:%d", *actual.Pdid)
	// TODO: decode RMI data ?
	case "CnRmiRequestType":
		actual := message.OperationType.(*proto.CnRmiRequest)
		log.Debugf("Received Rmi request for node:%d with data:%x", *actual.NodeId, actual.Message)
	case "CnRmiResponseType":
		actual := message.OperationType.(*proto.CnRmiResponse)
		log.Debugf("Received Rmi response with result:%d and data:%x", *actual.Result, actual.Message)
	case "CnRmiAsyncRequestType":
		actual := message.OperationType.(*proto.CnRmiAsyncRequest)
		log.Debugf("Received Rmi async request for node:%d and data:%x", *actual.NodeId, actual.Message)
	case "CnRmiAsyncResponseType":
		actual := message.OperationType.(*proto.CnRmiAsyncResponse)
		log.Debugf("Received Rmi async response with result:%d and data:%x", *actual.Result, actual.Message)
	}
	return message, nil
}

func (m Message) String() string {
	return fmt.Sprintf("Src=%x; Dst=%x; Cmd_type=%v; ref=%v; RawMessage=%x", m.Src, m.Dst, m.Operation.Type.String(), *m.Operation.Reference, m.RawMessage)
}

// creates the correct response message as a byte slice, for the parent message
func (m Message) CreateResponse(status proto.GatewayOperation_GatewayResult) []byte {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Message",
		"method": "CreateResponse",
	})

	log.Debugf("creating response for operation type: %s", reflect.TypeOf(m.OperationType).Elem().Name())
	responseType := getResponseTypeForOperationType(m.OperationType)
	operation := proto.GatewayOperation{
		Type:      &responseType,
		Reference: m.Operation.Reference,
		Result:    &status,
	}
	if status == -1 {
		operation.Result = nil
	}

	responseStruct := getStructForType(responseType.String())
	if responseStruct == nil {
		log.Errorf("unable to find struct for type: %s", responseType.String())
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

	return m.packMessage(operation, responseStruct)
}

func (m Message) CreateCustomResponse(operationType proto.GatewayOperation_OperationType, operationTypeStruct OperationType) []byte {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"object": "Message",
		"method": "CreateCustomResponse",
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
	log.Debugf("operationBytes: %x", operationBytes)
	operationTypeBytes, _ := operationType.XXX_Marshal(nil, false)
	log.Debugf("operationTypeBytes: %x", operationTypeBytes)
	response := make([]byte, 4)
	binary.BigEndian.PutUint32(response, uint32(len(operationTypeBytes)+34+len(operationBytes))) // raw message length
	log.Debugf("length: %x", response)
	response = append(response, m.Dst...)
	response = append(response, m.Src...)
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(len(operationBytes))) // op length
	log.Debugf("op length: %x", b)
	response = append(response, b...)
	response = append(response, operationBytes...) // gatewayOperation
	response = append(response, operationTypeBytes...)

	return response
}

func (m Message) Encode() []byte {
	return m.packMessage(m.Operation, m.OperationType)
}

// take an IP address, and a MAC address to respond with and create search gateway response
func CreateSearchGatewayResponse(ipAddress string, macAddress []byte) []byte {
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

// takes the name for an operation type and finds the struct for it
func getStructForType(operationTypeString string) OperationType {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "getStructForType",
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

func readBytes(conn net.Conn, size int) ([]byte, error) {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "readBytes",
	})

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
			log.Debugf("read result now: %x, read bytes:%d", result, readLen)
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
