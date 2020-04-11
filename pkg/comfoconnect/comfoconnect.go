package comfoconnect

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/proto"
)

type OperationTyp interface {
	XXX_Unmarshal([]byte) error
	XXX_Marshal(b []byte, deterministic bool) ([]byte, error)
}

type Message struct {
	length        uint32
	src           []byte
	dst           []byte
	Operation     proto.GatewayOperation
	rawMessage    []byte
	OperationType OperationTyp
}

func NewMessage(b []byte) Message {
	operation := proto.GatewayOperation{}
	operation.XXX_Unmarshal(b[38:42]) // we assume here that this is always a gateway operation, else we'd have to read the operation length from [36:38]

	operationType := getStructForType(operation.Type.String())
	operationType.XXX_Unmarshal(b[42:])

	m := Message{
		length:        binary.BigEndian.Uint32(b[:4]),
		src:           b[4:20],
		dst:           b[20:36],
		Operation:     operation,
		rawMessage:    b[42:],
		OperationType: operationType,
	}

	return m
}

func (m Message) String() string {
	return fmt.Sprintf("src=%x; dst=%x; Cmd_type=%v; ref=%v; rawMessage=%x", m.src, m.dst, m.Operation.Type.String(), *m.Operation.Reference, m.rawMessage)
}

func (m Message) CreateResponse(status proto.GatewayOperation_GatewayResult) []byte {
	logrus.Debugf("creating response for operation type: %s", reflect.TypeOf(m.OperationType).Elem().Name())
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
		logrus.Errorf("unable to find struct for type: %s", responseType.String())
		return nil
	}

	// start building up the response byte slice
	operationBytes, _ := operation.XXX_Marshal(nil, false)
	operationTypeBytes, _ := responseStruct.XXX_Marshal(nil, false)
	response := make([]byte, 4)
	binary.BigEndian.PutUint32(response, uint32(len(operationTypeBytes)+34+len(operationBytes))) // raw message length
	response = append(response, m.dst...)
	response = append(response, m.src...)
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(len(operationBytes))) // op length
	response = append(response, b...)
	response = append(response, operationBytes...) // gatewayOperation
	response = append(response, operationTypeBytes...)

	return response
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

func getStructForType(operationTypeString string) OperationTyp {
	var operationType OperationTyp
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
		logrus.Errorf("unable to find matching struct for operation type: %s", operationTypeString)
	} else {
		logrus.Debugf("found struct: %s, for operation type:%s", reflect.TypeOf(operationType).Elem().Name(), operationTypeString)
	}
	return operationType
}

func getResponseTypeForOperationType(operationType OperationTyp) proto.GatewayOperation_OperationType {
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
		logrus.Errorf("unable to find response type for operation type: %s", operationTypeString)
	} else {
		logrus.Debugf("found response type: %s for operation type: %s", responseTypeString, operationTypeString)
	}
	return responseTypeString
}
