package comfoconnect

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.TraceLevel)
	code := m.Run()
	os.Exit(code)
}
