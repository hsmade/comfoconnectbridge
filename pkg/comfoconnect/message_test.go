package comfoconnect

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/hsmade/comfoconnectbridge/pb"
)

func TestMessage_String(t *testing.T) {
	CnRpdoRequestType := pb.GatewayOperation_CnRpdoRequestType
	reference := uint32(123)
	one := uint32(1)
	timeout := uint32(16777215)
	pdid := uint32(58)

	type fields struct {
		Src           []byte
		Dst           []byte
		Operation     *pb.GatewayOperation
		RawMessage    []byte
		OperationType OperationType
		Span          opentracing.Span
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "happy path",
			fields: fields{
				Src: []byte{0x01, 0x02},
				Dst: []byte{0x03, 0x04},
				Operation: &pb.GatewayOperation{
					Type:      &CnRpdoRequestType,
					Reference: &reference,
				},
				OperationType: &pb.CnRpdoRequest{
					Pdid:    &pdid,
					Zone:    &one,
					Type:    &one,
					Timeout: &timeout,
				},
				RawMessage: []byte{0x05, 0x06},
				Span:       nil,
			},
			want: "Src=0102; Dst=0304; Opr_type=CnRpdoRequestType; ref=123; Opr:=pdid:58 zone:1 type:1 timeout:16777215",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Message{
				Src:           tt.fields.Src,
				Dst:           tt.fields.Dst,
				Operation:     tt.fields.Operation,
				RawMessage:    tt.fields.RawMessage,
				OperationType: tt.fields.OperationType,
				Span:          tt.fields.Span,
			}
			if got := strings.Replace(m.String(), "  ", " ", -1); got != tt.want {
				t.Errorf("String() = \ngot:  %v\nwant: %v", got, tt.want)
			}
		})
	}
}

func TestGetMessageFromSocket(t *testing.T) {
	CnRpdoRequestType := pb.GatewayOperation_CnRpdoRequestType
	closeSessionType := pb.GatewayOperation_CloseSessionRequestType
	reference := uint32(123)
	one := uint32(1)
	timeout := uint32(16777215)
	pdid := uint32(58)

	tests := []struct {
		name    string
		input   []byte
		want    *Message
		wantErr bool
	}{
		{
			name: "happy path: operation with data",
			input: []byte{ // 00 00 00 32 326b8252aa474fcab197a87f4d114ca300000000003310138001144fd71e23e6 0005 082620cd04083a1001180120ffffff07
				0x00, 0x00, 0x00, 0x31, // length
				0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6, // src
				0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12, // dst
				0x00, 0x04, // operation type length
				0x08, 0x26, 0x20, // operation type
				0x7b,                                                             // ref
				0x08, 0x3a, 0x10, 0x01, 0x18, 0x01, 0x20, 0xff, 0xff, 0xff, 0x07, // data
			},
			want: &Message{
				Src: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6},
				Dst: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12},
				Operation: &pb.GatewayOperation{
					Type:      &CnRpdoRequestType,
					Reference: &reference,
				},
				OperationType: &pb.CnRpdoRequest{
					Pdid:    &pdid,
					Zone:    &one,
					Type:    &one,
					Timeout: &timeout,
				},
			},
			wantErr: false,
		},
		{
			name: "happy path: operation without data",
			input: []byte{
				0x00, 0x00, 0x00, 0x26, // length
				0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6, // src
				0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12, // dst
				0x00, 0x04, // operation type length
				0x08, 0x04, 0x20, // operation type
				0x7b, // ref
			},
			want: &Message{
				Src: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6},
				Dst: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12},
				Operation: &pb.GatewayOperation{
					Type:      &closeSessionType,
					Reference: &reference,
				},
				OperationType: &pb.CloseSessionRequest{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := net.Pipe()
			go func() {
				_, _ = server.Write(tt.input)
				_ = server.Close()
			}()
			got, err := NewMessageFromSocket(client)
			_ = client.Close()

			if got != nil {
				got.Span = nil
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMessageFromSocket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, fmt.Sprint(got), fmt.Sprint(tt.want))
			//if !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("NewMessageFromSocket() \ngot: \n\t%s\n, want: \n\t%s", got, tt.want)
			//}
		})
	}
}

func TestMessage_Send(t *testing.T) {
	closeSessionType := pb.GatewayOperation_CloseSessionRequestType
	reference := uint32(123)

	type fields struct {
		Src           []byte
		Dst           []byte
		Operation     *pb.GatewayOperation
		RawMessage    []byte
		OperationType OperationType
		Span          opentracing.Span
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		result  []byte
	}{
		{
			name: "happy path",
			fields: fields{
				Src: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6},
				Dst: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12},
				Operation: &pb.GatewayOperation{
					Type:      &closeSessionType,
					Reference: &reference,
				},
				OperationType: &pb.CloseSessionRequest{},
			},
			wantErr: false,
			result: []byte{
				0x00, 0x00, 0x00, 0x26, // length
				0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6, // src
				0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12, // dst
				0x00, 0x04, // operation type length
				0x08, 0x04, 0x20, // operation type
				0x7b, // ref
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{
				Src:           tt.fields.Src,
				Dst:           tt.fields.Dst,
				Operation:     tt.fields.Operation,
				RawMessage:    tt.fields.RawMessage,
				OperationType: tt.fields.OperationType,
				Span:          tt.fields.Span,
			}
			logrus.SetLevel(logrus.TraceLevel)

			server, client := net.Pipe()
			go func() {
				if err := m.Send(server); (err != nil) != tt.wantErr {
					t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
				}
			}()

			got := make([]byte, len(tt.result))
			_ = client.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, err := client.Read(got)
			if err != nil {
				t.Errorf("error while receiving: %v", err)
			}

			_ = server.Close()
			_ = client.Close()
			assert.Equal(t, tt.result, got)
		})
	}
}

func TestMessage_Encode(t *testing.T) {
	closeSessionType := pb.GatewayOperation_CloseSessionRequestType
	reference := uint32(123)

	type fields struct {
		Src           []byte
		Dst           []byte
		Operation     *pb.GatewayOperation
		RawMessage    []byte
		OperationType OperationType
		Span          opentracing.Span
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
	}{
		{
			name: "happy path",
			fields: fields{
				Src: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6},
				Dst: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12},
				Operation: &pb.GatewayOperation{
					Type:      &closeSessionType,
					Reference: &reference,
				},
				OperationType: &pb.CloseSessionRequest{},
			},
			want: []byte{
				0x00, 0x00, 0x00, 0x26, // length
				0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6, // src
				0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12, // dst
				0x00, 0x04, // operation type length
				0x08, 0x04, 0x20, // operation type
				0x7b, // ref
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Message{
				Src:           tt.fields.Src,
				Dst:           tt.fields.Dst,
				Operation:     tt.fields.Operation,
				RawMessage:    tt.fields.RawMessage,
				OperationType: tt.fields.OperationType,
				Span:          tt.fields.Span,
			}
			if got := m.Encode(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Encode() = %v, want %v", got, tt.want)
			}
		})
	}
}
