package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/instrumentation"
	"github.com/hsmade/comfoconnectbridge/pkg/proxy"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = time.StampMilli
	logrus.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	closer := instrumentation.EnableTracing("proxy", "tower:5775")
	defer closer.Close()
	instrumentation.EnableMetrics()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	l := comfoconnect.NewBroadcastListener("192.168.178.52", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12})
	go l.Run()
	defer l.Stop()

	p := proxy.NewProxy("192.168.0.19", []byte{0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12})
	go p.Run(ctx, wg)

	logrus.Info("waiting for ctrl-c")
	for range c {
		logrus.Info("closing down")
		l.Stop()
		cancel()
		wg.Wait()
		os.Exit(0)
	}
}
