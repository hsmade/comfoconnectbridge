package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/hsmade/comfoconnectbridge/pb"
	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/helpers"
)

var (
	metricsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "comfoconnect_pdo_value",
			Help: "Value for the different PDOs, as they're seen by the proxy",
		},
		[]string{"ID", "description"},
	)
)

type Client struct {
	GatewayIP        string
	GatewayUUID      []byte
	conn             net.Conn
	DeviceName       string
	reference        uint32
	Pin              uint32
	MyUUID           []byte
	Sensors          []Sensor
	ReceivedMessages chan *comfoconnect.Message
	sendLock         sync.Mutex
}

type Sensor struct {
	Ppid uint32
	Type uint32
}

func (c Client) Run(ctx context.Context) {
	prometheus.MustRegister(metricsGauge)

	for { // restart when we loose the session
		select {
		case <-ctx.Done():
			return
		default:
			c.runSession(ctx)
		}
	}
}

func (c *Client) sendMessage(message *comfoconnect.Message) error {
	if c.reference == 0 {
		c.reference = 1 // FIXME: need NewClient() to solve this in a better way
	}
	c.sendLock.Lock()
	err := message.Send(c.conn)
	c.reference++
	if c.reference > 1024 {
		c.reference = 1
	}
	c.sendLock.Unlock()
	return err
}

func (c *Client) runSession(ctx context.Context) {
	connection, err := net.Dial("tcp", fmt.Sprintf("%s:56747", c.GatewayIP))
	if err != nil {
		helpers.StackLogger().Errorf("connect to gw: %v", err)
		time.Sleep(5 * time.Second) // we don't want to spam the gateway
		return
	}
	helpers.StackLogger().Infof("connected to %s", connection.RemoteAddr())
	c.conn = connection

	err = c.registerClient()
	if err != nil {
		helpers.StackLogger().Errorf("registering with gw: %v", err)
		return
	}

	err = c.requestNewSession()
	if err != nil {
		helpers.StackLogger().Errorf("session request with gw: %v", err)
		return
	}

	go c.keepAliveFunc(ctx) // ping the gateway every now and then, to keep the session alive

	err = c.subscribeAll()
	if err != nil {
		helpers.StackLogger().Errorf("subscribeAll with gw: %v", err)
		return
	}

	// endless loop of receiving messages
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, err := c.receiveMessage()
			if err != nil {
				helpers.StackLogger().Errorf("receiveMessage from gw: %v", err)
				return
			}
			if c.ReceivedMessages != nil {
				c.ReceivedMessages <- message
			}
		}
	}
}

func (c *Client) registerClient() error {
	c.reference++
	operationType := pb.GatewayOperation_RegisterAppRequestType
	message := &comfoconnect.Message{
		Src: c.MyUUID,
		Dst: c.GatewayUUID,
		Operation: &pb.GatewayOperation{
			Type:      &operationType,
			Reference: &c.reference,
		},
		OperationType: &pb.RegisterAppRequest{
			Uuid:       c.MyUUID,
			Pin:        &c.Pin,
			Devicename: &c.DeviceName,
		},
	}

	helpers.StackLogger().Debugf("Writing RegisterAppRequest: %x", message.Encode())
	err := c.sendMessage(message)
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "sending RegisterAppRequest"))
	}

	// receiveMessage the confirmation for the registration
	helpers.StackLogger().Debugf("receiving RegisterAppConfirm")
	message, err = c.receiveMessage()
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "receiving RegisterAppConfirm"))
	}
	if message.Operation.Type.String() != "RegisterAppConfirmType" {
		return helpers.LogOnError(errors.New(fmt.Sprintf("received invalid message type instead of RegisterAppConfirmType: %v", message.String())))
	}
	helpers.StackLogger().Debugf("received RegisterAppConfirm: %x", message.Encode())

	return nil
}

func (c *Client) requestNewSession() error {
	// send a start session request
	operationType := pb.GatewayOperation_StartSessionRequestType
	message := &comfoconnect.Message{
		Src: c.MyUUID,
		Dst: c.GatewayUUID,
		Operation: &pb.GatewayOperation{
			Type:      &operationType,
			Reference: &c.reference,
		},
		OperationType: &pb.StartSessionRequest{},
	}
	err := c.sendMessage(message)
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "sending StartSessionRequest"))
	}

	// receiveMessage the confirmation for the session
	message, err = c.receiveMessage()
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "receiving StartSessionConfirm"))
	}
	if message.Operation.Type.String() != "StartSessionConfirmType" {
		return helpers.LogOnError(errors.New(fmt.Sprintf("received invalid message type instead of StartSessionConfirmType: %v", message.String())))
	}
	result := *message.Operation.Result
	if result != pb.GatewayOperation_OK {
		return helpers.LogOnError(errors.New(fmt.Sprintf("failed to start new session. Gateway responded with: %s", result.String())))
	}

	return nil
}

// ping gateway to keep session alive
func (c *Client) keepAliveFunc(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			helpers.StackLogger().Debug("sending keep alive")
			operationType := pb.GatewayOperation_CnTimeRequestType
			message := comfoconnect.Message{
				Src: c.MyUUID,
				Dst: c.GatewayUUID,
				Operation: &pb.GatewayOperation{
					Type:      &operationType,
					Reference: &c.reference,
				},
				OperationType: &pb.CnTimeRequest{},
			}
			err := c.sendMessage(&message)
			if err != nil {
				if errors.Cause(err) == io.EOF {
					helpers.StackLogger().Debug("Connection closed, stopping keep alives")
					return
				}
				helpers.StackLogger().Errorf("keepalive got error: %v", err)
			}
		}
	}
}

// Subscribe to all known PDIDs (c.Sensors)
func (c *Client) subscribeAll() error {
	for _, sensor := range c.Sensors {
		helpers.StackLogger().Debugf("subscribing to ppid:%d and type:%d", sensor.Ppid, sensor.Type)
		err := c.subscribe(sensor.Ppid, sensor.Type)
		if err != nil {
			helpers.StackLogger().Errorf("failed to subscribe to Sensor (ppid=%d, type=%d): %v", sensor.Ppid, sensor.Type, err)
			//return err
			continue
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func (c *Client) subscribe(pdid uint32, pType uint32) error {
	helpers.StackLogger().Debug("subscribing to pdid:%d, type:%d", pdid, pType)
	operationType := pb.GatewayOperation_CnRpdoRequestType
	zone := uint32(1)
	message := &comfoconnect.Message{
		Src: c.MyUUID,
		Dst: c.GatewayUUID,
		Operation: &pb.GatewayOperation{
			Type:      &operationType,
			Reference: &c.reference,
		},
		OperationType: &pb.CnRpdoRequest{
			Pdid: &pdid,
			Zone: &zone,
			Type: &pType,
		},
	}
	err := c.sendMessage(message)
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "requesting RPDO"))
	}
	helpers.StackLogger().Infof("subscribed to RPDO with reference: %d", c.reference)

	for i := 0; i < 10; i++ {
		message, err = c.receiveMessage()
		if err != nil {
			return helpers.LogOnError(errors.Wrap(err, "receiving CnRpdoConfirm"))
		}
		helpers.StackLogger().Debugf("received: %v with err=%v", message, err)
		if message.Operation.Type.String() == "CnRpdoConfirmType" {
			helpers.StackLogger().Debugf("subscription confirmed after %d times", i+1)
			return nil
		}
	}
	return helpers.LogOnError(errors.New("Failed to receiveMessage CnRpdoConfirm"))
}

func (c *Client) receiveMessage() (*comfoconnect.Message, error) {
	message, err := comfoconnect.NewMessageFromSocket(c.conn)
	if err == nil {
		helpers.StackLogger().Infof("received %v from %s", message, c.conn.RemoteAddr().String())
		if message.Operation.Type != nil {
			generateMetrics(message)
		}
	} else {
		if errors.Cause(err) == io.EOF {
			return nil, helpers.LogOnError(errors.Wrap(err, "client left"))
		}
		helpers.StackLogger().Debugf("receiveMessage err: %v", err)
	}
	return message, helpers.LogOnError(err)
}

func generateMetrics(message *comfoconnect.Message) {
	switch message.Operation.Type.String() {
	case "CnRpdoNotificationType":
		conv := message.DecodePDO()
		helpers.StackLogger().Infof("Got RPDO: %s %v with value %f", reflect.TypeOf(conv), conv, conv.Tofloat64())
		metricsGauge.WithLabelValues(conv.GetID(), conv.GetDescription()).Set(conv.Tofloat64())
	case "CnAlarmNotificationType":
		helpers.StackLogger().Warnf("Got alarm notification: %v", message)
	}
	helpers.StackLogger().Debugf("called for %v", message)
}
