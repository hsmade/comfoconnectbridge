package main

import (
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/bridge"
	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	//logrus.SetReportCaller(true)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	//l := bridge.NewBroadcastListener("192.168.178.2", []byte{0x88, 0xe9, 0xfe, 0x51, 0xc5, 0x46})
	l := comfoconnect.NewBroadcastListener("192.168.178.21", []byte{0x70, 0x85, 0xc2, 0xb7, 0x8c, 0xa0})
	go l.Run()
	defer l.Stop()

	b := bridge.NewBridge("192.168.178.21", "")
	go b.Run()
	defer b.Stop()

	logrus.Info("waiting for ctrl-c")
	for _ = range c {
		logrus.Info("closing down")
		l.Stop()
		b.Stop()
		os.Exit(0)
	}
}
