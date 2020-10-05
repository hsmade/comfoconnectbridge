clean:
	rm zehnder.proto zehnder.pb.go

zehnder.proto :
	echo 'syntax = "proto2";' > zehnder.proto
	echo 'option go_package = "github.com/hsmade/comfoconnectbridge/proto";' >> zehnder.proto
	curl -s https://raw.githubusercontent.com/michaelarnauts/comfoconnect/master/protobuf/zehnder.proto >> zehnder.proto

deps:
	go get github.com/golang/protobuf/protoc-gen-go

zehnder.pb.go : zehnder.proto deps
	protoc --proto_path=. --go_out=. --go_opt=paths=source_relative zehnder.proto

all: zehnder.pb.go