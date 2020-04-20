package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/proxy"
	"github.com/hsmade/comfoconnectbridge/pkg/tracing"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	//logrus.SetReportCaller(true)
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = time.StampMilli
	logrus.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	tracer, closer := tracing.New("proxy")
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	l := comfoconnect.NewBroadcastListener("192.168.178.52", []byte{0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12})
	go l.Run()
	defer l.Stop()

	p := proxy.NewProxy("192.168.178.2", []byte{0xb8, 0x27, 0xeb, 0xf9, 0xf9, 0x12})
	go p.Run(ctx, wg)

	logrus.Info("waiting for ctrl-c")
	for _ = range c {
		logrus.Info("closing down")
		l.Stop()
		cancel()
		wg.Wait()
		os.Exit(0)
	}
}
