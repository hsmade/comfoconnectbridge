package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/dumbproxy"
	"github.com/hsmade/comfoconnectbridge/pkg/helpers"
	"github.com/hsmade/comfoconnectbridge/pkg/instrumentation"
)

func main() {
	logrus.SetLevel(logrus.TraceLevel)
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = time.StampMilli
	logrus.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	closer := instrumentation.EnableTracing("proxy", "tower:5775")
	defer closer.Close()
	//instrumentation.EnableMetrics()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	gatewayUUID, err := comfoconnect.DiscoverGateway("192.168.0.19")
	if err != nil {
		helpers.StackLogger().Errorf("failed to discover gateway: %v", err)
		return
	}

	helpers.StackLogger().Infof("got gateway UUID: %x", gatewayUUID)

	l := comfoconnect.NewBroadcastListener("192.168.178.52", gatewayUUID)
	go l.Run()
	defer l.Stop()

	logFile, err := os.OpenFile("dumbproxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	p := dumbproxy.DumbProxy{
		GatewayIP: "192.168.0.19",
		LogFile: logFile,
	}
	go p.Run(ctx, wg)

	helpers.StackLogger().Info("waiting for ctrl-c")
	for range c {
		helpers.StackLogger().Info("closing down")
		l.Stop()
		cancel()
		wg.Wait()
		os.Exit(0)
	}
}
