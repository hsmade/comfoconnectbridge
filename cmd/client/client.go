package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/client"
	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/instrumentation"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = time.StampMilli
	logrus.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	ctx, cancel := context.WithCancel(context.Background())

	//closer := instrumentation.EnableTracing("proxy", "tower:5775")
	//defer closer.Close()
	instrumentation.EnableMetrics()

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)

	GatewayIP := "192.168.0.19"
	// first ping the gateway to get its UUID
	GatewayUUID, err := comfoconnect.DiscoverGateway(GatewayIP)
	if err != nil {
		log.Fatalf("failed to discover gateway: %v", err)
	}

	c := client.Client{
		GatewayIP:   GatewayIP,
		GatewayUUID: GatewayUUID,
		DeviceName:  "client",
		Pin:         0,
		MyUUID:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12},
		Sensors: []client.Sensor{
			// got this from decoding what the app asks for
			{16, 1},
			{33, 1},
			{37, 1},
			{42, 1},
			{49, 1},
			{53, 1},
			{56, 1},
			{57, 1},
			{58, 1},
			{65, 1},
			{66, 1},
			{67, 1},
			{70, 1},
			{71, 1},
			{73, 1},
			{74, 1},
			{81, 3},
			{82, 3},
			{85, 3},
			{86, 3},
			{87, 3},
			{89, 3},
			{90, 3},
			{117, 1},
			{118, 1},
			{119, 2},
			{120, 2},
			{121, 2},
			{122, 2},
			{128, 2},
			{129, 2},
			{130, 2},
			{144, 2},
			{145, 2},
			{146, 2},
			{176, 1},
			{192, 2},
			{208, 1},
			{209, 6},
			{210, 0},
			{211, 0},
			{212, 6},
			{213, 2},
			{214, 2},
			{215, 2},
			{216, 2},
			{217, 2},
			{218, 2},
			{219, 2},
			{220, 6},
			{221, 6},
			{224, 1},
			{225, 1},
			{226, 2},
			{227, 1},
			{228, 1},
			{230, 8},
			{274, 6},
			{275, 6},
			{278, 6},
			{290, 1},
			{291, 1},
			{292, 1},
			{294, 1},
			{321, 2},
			{325, 2},
			{330, 2},
			{337, 3},
			{338, 3},
			{341, 3},
			{345, 3},
			{346, 3},
			{369, 1},
			{370, 1},
			{371, 1},
			{372, 1},
			{384, 6},
			{386, 0},
			{400, 6},
			{401, 1},
			{402, 0},
			{416, 6},
			{417, 6},
			{418, 1},
			{419, 0},
			{784, 1},
			{802, 6},
		},
	}

	c.Run(ctx)

	logrus.Info("waiting for ctrl-signalChannel")
	for _ = range signalChannel {
		logrus.Info("closing down")
		cancel()
		os.Exit(0)
	}
}
