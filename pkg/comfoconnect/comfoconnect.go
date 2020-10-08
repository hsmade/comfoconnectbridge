package comfoconnect

import (
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pb"
)

// take an IP address, and a MAC address to respond with and create search gateway response
func CreateSearchGatewayResponse(ipAddress string, uuid []byte) []byte {
	version := uint32(1)
	//uuid := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01} // uuid header
	//uuid = append(uuid, macAddress...)

	resp := pb.SearchGatewayResponse{}

	_ = proto.Unmarshal([]byte{0x12, 0x24}, &resp)
	resp.Ipaddress = &ipAddress
	resp.Uuid = uuid
	resp.Version = &version
	b, _ := proto.Marshal(&resp)
	//b, _ := resp.XXX_Marshal([]byte{0x12, 0x24}, false)
	return b
}

// takes the name for an operation type and finds the struct for it
func GetStructForType(operationTypeString string) proto.Message {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "GetStructForType",
	})

	var operationType OperationType
	switch operationTypeString {
	case "SetAddressRequestType":
		operationType = &pb.SetAddressRequest{}
	case "RegisterAppRequestType":
		operationType = &pb.RegisterAppRequest{}
	case "StartSessionRequestType":
		operationType = &pb.StartSessionRequest{}
	case "CloseSessionRequestType":
		operationType = &pb.CloseSessionRequest{}
	case "ListRegisteredAppsRequestType":
		operationType = &pb.ListRegisteredAppsRequest{}
	case "DeregisterAppRequestType":
		operationType = &pb.DeregisterAppRequest{}
	case "ChangePinRequestType":
		operationType = &pb.ChangePinRequest{}
	case "GetRemoteAccessIdRequestType":
		operationType = &pb.GetRemoteAccessIdRequest{}
	case "SetRemoteAccessIdRequestType":
		operationType = &pb.SetRemoteAccessIdRequest{}
	case "GetSupportIdRequestType":
		operationType = &pb.GetSupportIdRequest{}
	case "SetSupportIdRequestType":
		operationType = &pb.SetSupportIdRequest{}
	case "GetWebIdRequestType":
		operationType = &pb.GetWebIdRequest{}
	case "SetWebIdRequestType":
		operationType = &pb.SetWebIdRequest{}
	case "SetPushIdRequestType":
		operationType = &pb.SetPushIdRequest{}
	case "DebugRequestType":
		operationType = &pb.DebugRequest{}
	case "UpgradeRequestType":
		operationType = &pb.UpgradeRequest{}
	case "SetDeviceSettingsRequestType":
		operationType = &pb.SetDeviceSettingsRequest{}
	case "VersionRequestType":
		operationType = &pb.VersionRequest{}
	case "SetAddressConfirmType":
		operationType = &pb.SetAddressConfirm{}
	case "RegisterAppConfirmType":
		operationType = &pb.RegisterAppConfirm{}
	case "StartSessionConfirmType":
		operationType = &pb.StartSessionConfirm{}
	case "CloseSessionConfirmType":
		operationType = &pb.CloseSessionConfirm{}
	case "ListRegisteredAppsConfirmType":
		operationType = &pb.ListRegisteredAppsConfirm{}
	case "DeregisterAppConfirmType":
		operationType = &pb.DeregisterAppConfirm{}
	case "ChangePinConfirmType":
		operationType = &pb.ChangePinConfirm{}
	case "GetRemoteAccessIdConfirmType":
		operationType = &pb.GetRemoteAccessIdConfirm{}
	case "SetRemoteAccessIdConfirmType":
		operationType = &pb.SetRemoteAccessIdConfirm{}
	case "GetSupportIdConfirmType":
		operationType = &pb.GetSupportIdConfirm{}
	case "SetSupportIdConfirmType":
		operationType = &pb.SetSupportIdConfirm{}
	case "GetWebIdConfirmType":
		operationType = &pb.GetWebIdConfirm{}
	case "SetWebIdConfirmType":
		operationType = &pb.SetWebIdConfirm{}
	case "SetPushIdConfirmType":
		operationType = &pb.SetPushIdConfirm{}
	case "DebugConfirmType":
		operationType = &pb.DebugConfirm{}
	case "UpgradeConfirmType":
		operationType = &pb.UpgradeConfirm{}
	case "SetDeviceSettingsConfirmType":
		operationType = &pb.SetDeviceSettingsConfirm{}
	case "VersionConfirmType":
		operationType = &pb.VersionConfirm{}
	case "GatewayNotificationType":
		operationType = &pb.GatewayNotification{}
	case "KeepAliveType":
		operationType = &pb.KeepAlive{}
	case "FactoryResetType":
		operationType = &pb.FactoryReset{}
	case "CnTimeRequestType":
		operationType = &pb.CnTimeRequest{}
	case "CnTimeConfirmType":
		operationType = &pb.CnTimeConfirm{}
	case "CnNodeRequestType":
		operationType = &pb.CnNodeRequest{}
	case "CnNodeNotificationType":
		operationType = &pb.CnNodeNotification{}
	case "CnRmiRequestType":
		operationType = &pb.CnRmiRequest{}
	case "CnRmiResponseType":
		operationType = &pb.CnRmiResponse{}
	case "CnRmiAsyncRequestType":
		operationType = &pb.CnRmiAsyncRequest{}
	case "CnRmiAsyncConfirmType":
		operationType = &pb.CnRmiAsyncConfirm{}
	case "CnRmiAsyncResponseType":
		operationType = &pb.CnRmiAsyncResponse{}
	case "CnRpdoRequestType":
		operationType = &pb.CnRpdoRequest{}
	case "CnRpdoConfirmType":
		operationType = &pb.CnRpdoConfirm{}
	case "CnRpdoNotificationType":
		operationType = &pb.CnRpdoNotification{}
	case "CnAlarmNotificationType":
		operationType = &pb.CnAlarmNotification{}
	case "CnFupReadRegisterRequestType":
		operationType = &pb.CnFupReadRegisterRequest{}
	case "CnFupReadRegisterConfirmType":
		operationType = &pb.CnFupReadRegisterConfirm{}
	case "CnFupProgramBeginRequestType":
		operationType = &pb.CnFupProgramBeginRequest{}
	case "CnFupProgramBeginConfirmType":
		operationType = &pb.CnFupProgramBeginConfirm{}
	case "CnFupProgramRequestType":
		operationType = &pb.CnFupProgramRequest{}
	case "CnFupProgramConfirmType":
		operationType = &pb.CnFupProgramConfirm{}
	case "CnFupProgramEndRequestType":
		operationType = &pb.CnFupProgramEndRequest{}
	case "CnFupProgramEndConfirmType":
		operationType = &pb.CnFupProgramEndConfirm{}
	case "CnFupReadRequestType":
		operationType = &pb.CnFupReadRequest{}
	case "CnFupReadConfirmType":
		operationType = &pb.CnFupReadConfirm{}
	case "CnFupResetRequestType":
		operationType = &pb.CnFupResetRequest{}
	case "CnFupResetConfirmType":
		operationType = &pb.CnFupResetConfirm{}
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

// takes an operation type and finds the correct type to respond with
func getResponseTypeForOperationType(operationType OperationType) pb.GatewayOperation_OperationType {
	log := logrus.WithFields(logrus.Fields{
		"module": "comfoconnect",
		"method": "getResponseTypeForOperationType",
	})

	var responseTypeString pb.GatewayOperation_OperationType
	operationTypeString := reflect.TypeOf(operationType).Elem().Name()

	switch operationTypeString {
	case "SetAddressRequest":
		responseTypeString = pb.GatewayOperation_SetAddressConfirmType
	case "RegisterAppRequest":
		responseTypeString = pb.GatewayOperation_RegisterAppConfirmType
	case "StartSessionRequest":
		responseTypeString = pb.GatewayOperation_StartSessionConfirmType
	case "CloseSessionRequest":
		responseTypeString = pb.GatewayOperation_CloseSessionConfirmType
	case "ListRegisteredAppsRequest":
		responseTypeString = pb.GatewayOperation_ListRegisteredAppsConfirmType
	case "DeregisterAppRequest":
		responseTypeString = pb.GatewayOperation_SetAddressConfirmType
	case "ChangePinRequest":
		responseTypeString = pb.GatewayOperation_ChangePinConfirmType
	case "GetRemoteAccessIdRequest":
		responseTypeString = pb.GatewayOperation_GetRemoteAccessIdConfirmType
	case "SetRemoteAccessIdRequest":
		responseTypeString = pb.GatewayOperation_SetRemoteAccessIdConfirmType
	case "GetSupportIdRequest":
		responseTypeString = pb.GatewayOperation_GetSupportIdConfirmType
	case "GetWebIdRequest":
		responseTypeString = pb.GatewayOperation_GetWebIdConfirmType
	case "SetWebIdRequest":
		responseTypeString = pb.GatewayOperation_SetWebIdConfirmType
	case "SetPushIdRequest":
		responseTypeString = pb.GatewayOperation_SetPushIdConfirmType
	case "DebugRequest":
		responseTypeString = pb.GatewayOperation_DebugConfirmType
	case "UpgradeRequest":
		responseTypeString = pb.GatewayOperation_UpgradeConfirmType
	case "SetDeviceSettingsRequest":
		responseTypeString = pb.GatewayOperation_SetDeviceSettingsConfirmType
	case "VersionRequest":
		responseTypeString = pb.GatewayOperation_VersionConfirmType
	case "CnTimeRequest":
		responseTypeString = pb.GatewayOperation_CnTimeConfirmType
	case "CnRmiRequest":
		responseTypeString = pb.GatewayOperation_CnRmiResponseType
	case "CnRmiAsyncRequest":
		responseTypeString = pb.GatewayOperation_CnRmiAsyncConfirmType
	case "CnRpdoRequest":
		responseTypeString = pb.GatewayOperation_CnRpdoConfirmType
	case "CnFupReadRegisterRequest":
		responseTypeString = pb.GatewayOperation_CnFupReadRegisterConfirmType
	case "CnFupProgramBeginRequest":
		responseTypeString = pb.GatewayOperation_CnFupProgramBeginConfirmType
	case "CnFupProgramRequest":
		responseTypeString = pb.GatewayOperation_CnFupProgramConfirmType
	case "CnFupProgramEndRequest":
		responseTypeString = pb.GatewayOperation_CnFupProgramEndConfirmType
	case "CnFupReadRequest":
		responseTypeString = pb.GatewayOperation_CnFupReadConfirmType
	case "CnFupResetRequest":
		responseTypeString = pb.GatewayOperation_CnFupResetConfirmType
	}

	if responseTypeString == 0 {
		log.Errorf("unable to find response type for operation type: %s", operationTypeString)
	} else {
		log.Debugf("found response type: %s for operation type: %s", responseTypeString, operationTypeString)
	}
	return responseTypeString
}

//func ReadFromSocket(conn net.Conn, data *[]byte, size int) error {
//	log := logrus.WithFields(logrus.Fields{
//		"module": "comfoconnect",
//		"method": "ReadFromSocket",
//		"size":   size,
//	})
//	log.Debugf("reading from %s", conn.RemoteAddr().String())
//	result := *data
//
//	for {
//		buffer := make([]byte, size)
//		readLen, err := conn.Read(buffer)
//		if err != nil {
//			return errors.Wrap(err, "reading from socket")
//		}
//
//		if readLen > 0 {
//			size -= readLen
//			result = append(result, buffer[:readLen]...)
//			log.Tracef("read result now: %x, read bytes:%d", result, readLen)
//		}
//
//		if size == 0 {
//			break
//		}
//
//		if size < 0 {
//			return errors.New("read too many bytes: size")
//		}
//	}
//
//	return nil
//}

// Read `size` bytes from a socket
// Will loop conn.Read() until it has enough bytes
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
		err := conn.SetReadDeadline(time.Now().Add(time.Second * 5))
		if err != nil {
			return nil, errors.Wrap(err, "setting timeout for read")
		}

		buffer := make([]byte, size)
		readLen, err := conn.Read(buffer)
		if err != nil {
			return result, errors.Wrap(err, "reading from socket")
		}

		if readLen > 0 {
			size -= readLen
			result = append(result, buffer[:readLen]...)
			log.Tracef("read result now: %x, read bytes:%d", result, readLen)
		}

		if size == 0 {
			break
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
