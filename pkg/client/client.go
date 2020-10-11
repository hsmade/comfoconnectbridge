package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
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
	GatewayIP   string
	GatewayUUID []byte
	conn        net.Conn
	DeviceName  string
	reference   uint32
	Pin         uint32
	MyUUID      []byte
	Sensors     []Sensor
}

type Sensor struct {
	Ppid uint32
	Type uint32
}

func (c Client) Run(ctx context.Context) {
	prometheus.MustRegister(metricsGauge)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.startSession(ctx)
		}
	}
}

func (c *Client) startSession(ctx context.Context) {
	connection, err := net.Dial("tcp", fmt.Sprintf("%s:56747", c.GatewayIP))
	if err != nil {
		helpers.StackLogger().Errorf("connect to gw: %v", err)
		//os.Exit(-1)
		time.Sleep(5 * time.Second)
		return
	}
	helpers.StackLogger().Infof("connected to %s", connection.RemoteAddr())
	c.conn = connection

	err = c.register()
	if err != nil {
		helpers.StackLogger().Errorf("registering with gw: %v", err)
		return
	}

	err = c.sessionRequest()
	if err != nil {
		helpers.StackLogger().Errorf("session request with gw: %v", err)
		return
	}

	go c.keepAlive(ctx)

	err = c.subscribeAll()
	if err != nil {
		helpers.StackLogger().Errorf("subscribeAll with gw: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, err := c.receive()
			if err != nil {
				helpers.StackLogger().Errorf("receive from gw: %v", err)
				return
			}
		}
	}
}

func (c *Client) register() error {
	c.reference++
	operationType := pb.GatewayOperation_RegisterAppRequestType
	m := &comfoconnect.Message{
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

	helpers.StackLogger().Debugf("Writing RegisterAppRequest: %x", m.Encode())
	_, err := c.conn.Write(m.Encode())
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "sending RegisterAppRequest"))
	}
	c.reference++

	// receive the confirmation for the registration
	helpers.StackLogger().Debugf("receiving RegisterAppConfirm")
	m, err = comfoconnect.NewMessageFromSocket(c.conn)
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "receiving RegisterAppConfirm"))
	}
	if m.Operation.Type.String() != "RegisterAppConfirmType" {
		return helpers.LogOnError(errors.New(fmt.Sprintf("received invalid message type instead of RegisterAppConfirmType: %v", m.String())))
	}
	helpers.StackLogger().Debugf("received RegisterAppConfirm: %x", m.Encode())

	return nil
}

func (c *Client) sessionRequest() error {
	// send a start session request
	c.reference++
	operationType := pb.GatewayOperation_StartSessionRequestType
	_, err := c.conn.Write(comfoconnect.Message{
		Src: c.MyUUID,
		Dst: c.GatewayUUID,
		Operation: &pb.GatewayOperation{
			Type:      &operationType,
			Reference: &c.reference,
		},
		OperationType: &pb.StartSessionRequest{},
	}.Encode())
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "sending StartSessionRequest"))
	}
	c.reference++

	// receive the confirmation for the session
	m, err := comfoconnect.NewMessageFromSocket(c.conn)
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "receiving StartSessionConfirm"))
	}
	if m.Operation.Type.String() != "StartSessionConfirmType" {
		return helpers.LogOnError(errors.New(fmt.Sprintf("received invalid message type instead of StartSessionConfirmType: %v", m.String())))
	}

	return nil
}

func (c *Client) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			helpers.StackLogger().Debug("sending keep alive")
			operationType := pb.GatewayOperation_CnTimeRequestType
			m := comfoconnect.Message{
				Src: c.MyUUID,
				Dst: c.GatewayUUID,
				Operation: &pb.GatewayOperation{
					Type:      &operationType,
					Reference: &c.reference,
				},
				OperationType: &pb.CnTimeRequest{},
			}
			_, err := c.conn.Write(m.Encode())
			if err != nil {
				if errors.Cause(err) == io.EOF {
					helpers.StackLogger().Debug("Connection closed, stopping keep alives")
					return
				}
				helpers.StackLogger().Errorf("keepalive got error: %v", err)
			}
			c.reference++
			if c.reference > 1024 {
				c.reference = 1
			}
		}
	}
}

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
	m := &comfoconnect.Message{
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
	_, err := c.conn.Write(m.Encode())
	if err != nil {
		return helpers.LogOnError(errors.Wrap(err, "requesting RPDO"))
	}
	helpers.StackLogger().Infof("subscribed to RPDO with reference: %d", c.reference)
	c.reference++

	for i := 0; i < 10; i++ {
		m, err = c.receive()
		if err != nil {
			return helpers.LogOnError(errors.Wrap(err, "receiving CnRpdoConfirm"))
		}
		helpers.StackLogger().Debugf("received: %v with err=%v", m, err)
		if m.Operation.Type.String() == "CnRpdoConfirmType" {
			helpers.StackLogger().Debugf("subscription confirmed after %d times", i+1)
			return nil
		}
	}
	return helpers.LogOnError(errors.New("Failed to receive CnRpdoConfirm"))
}

func (c *Client) receive() (*comfoconnect.Message, error) {
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
		helpers.StackLogger().Debugf("receive err: %v", err)
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
