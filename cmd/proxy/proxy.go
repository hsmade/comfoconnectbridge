package main

import (
	"os"
	"os/signal"
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

	tracer, closer := tracing.New("proxy")
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	l := comfoconnect.NewBroadcastListener("192.168.178.21", []byte{0x70, 0x85, 0xc2, 0xb7, 0x8c, 0xa0})
	go l.Run()
	defer l.Stop()

	p := proxy.NewProxy("192.168.178.21", []byte{0x70, 0x85, 0xc2, 0xb7, 0x8c, 0xa0})
	go p.Run()
	defer p.Stop()

	logrus.Info("waiting for ctrl-c")
	for _ = range c {
		logrus.Info("closing down")
		l.Stop()
		p.Stop()
		os.Exit(0)
	}
}
