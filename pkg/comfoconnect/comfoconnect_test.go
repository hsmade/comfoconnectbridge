package comfoconnect

import (
	"net"
	"reflect"
	"testing"
	"time"
)

func TestReadBytes(t *testing.T) {
	tests := []struct {
		name      string
		writeFunc func(conn net.Conn)
		size      int
		want      []byte
		wantErr   bool
	}{
		{
			name: "happy path: single write",
			writeFunc: func(conn net.Conn) {
				_, _ = conn.Write([]byte{0x01, 0x02})
			},
			size:    2,
			want:    []byte{0x01, 0x02},
			wantErr: false,
		},
		{
			name: "happy path: staggering write",
			writeFunc: func(conn net.Conn) {
				_, _ = conn.Write([]byte{0x01, 0x02})
				time.Sleep(100 * time.Millisecond)
				_, _ = conn.Write([]byte{0x03, 0x04})
			},
			size:    4,
			want:    []byte{0x01, 0x02, 0x03, 0x04},
			wantErr: false,
		},
		{
			name: "failure path: asking for size=0",
			writeFunc: func(conn net.Conn) {
				_, _ = conn.Write([]byte{0x01, 0x02})
			},
			size:    0,
			want:    nil,
			wantErr: true,
		},
		{
			name: "failure path: asking too many bytes / writing too little bytes",
			writeFunc: func(conn net.Conn) {
				_, _ = conn.Write([]byte{0x01, 0x02})
			},
			size:    20,
			want:    []byte{0x01, 0x02},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := net.Pipe()
			go tt.writeFunc(server)
			got, err := ReadBytes(client, tt.size)
			_ = server.Close()
			_ = client.Close()
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadBytes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateSearchGatewayResponse(t *testing.T) {
	type args struct {
		ipAddress string
		uuid      []byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "happy path",
			args: args{
				ipAddress: "127.0.0.1",
				uuid:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6},
			},
			want: []byte{0x0a, 0x09, 0x31, 0x32, 0x37, 0x2e, 0x30, 0x2e, 0x30, 0x2e, 0x31, 0x12, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x33, 0x10, 0x13, 0x80, 0x01, 0x14, 0x4f, 0xd7, 0x1e, 0x23, 0xe6, 0x18, 0x01},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateSearchGatewayResponse(tt.args.ipAddress, tt.args.uuid); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSearchGatewayResponse() = %x, want %x", got, tt.want)
			}
		})
	}
}
