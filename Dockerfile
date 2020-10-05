FROM golang
RUN apt update
RUN apt install -y protobuf-compiler/stable
COPY ./ /app/
WORKDIR /app/pb
RUN make all
WORKDIR /app
RUN GOOS=linux GOARCH=arm GOARM=7 go build cmd/client/client.go
RUN GOOS=linux GOARCH=arm GOARM=7 go build cmd/dumbproxy/dumbproxy.go
RUN GOOS=linux GOARCH=arm GOARM=7 go build cmd/proxy/proxy.go
RUN GOOS=linux GOARCH=arm GOARM=7 go build cmd/mockLanC/mockLanC.go