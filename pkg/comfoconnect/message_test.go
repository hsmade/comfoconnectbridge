package comfoconnect

import (
	"fmt"
	"net"
	"testing"

	"github.com/opentracing/opentracing-go"
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
			want: "Src=0102; Dst=0304; Opr_type=CnRpdoRequestType; ref=123; Opr:=pdid:58  zone:1  type:1  timeout:16777215",
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
			if got := m.String(); got != tt.want {
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
