package main

import (
	"os"
	"os/signal"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	//"github.com/hsmade/comfoconnectbridge/pkg/comfoconnect"
	"github.com/hsmade/comfoconnectbridge/pkg/mockLanC"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	//logrus.SetReportCaller(true)
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = time.StampMilli
	logrus.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	//l := bridge.NewBroadcastListener("192.168.178.2", []byte{0x88, 0xe9, 0xfe, 0x51, 0xc5, 0x46})
	//l := comfoconnect.NewBroadcastListener("192.168.178.21", []byte{0x70, 0x85, 0xc2, 0xb7, 0x8c, 0xa0})
	l := comfoconnect.NewBroadcastListener("192.168.178.21", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x25, 0x10, 0x10, 0x80, 0x01, 0xe2, 0x03, 0x56, 0x7c, 0x4d, 0x2c})
	go l.Run()
	defer l.Stop()

	b := mockLanC.NewMockLanC("192.168.178.21", "")
	go b.Run()
	defer b.Stop()

	time.Sleep(10 * time.Second)
	l.Stop()

	logrus.Info("waiting for ctrl-c")
	for range c {
		logrus.Info("closing down")
		l.Stop()
		b.Stop()
		os.Exit(0)
	}
}
