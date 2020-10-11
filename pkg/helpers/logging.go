package helpers

import (
	"fmt"
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
)

func StackLogger() *logrus.Entry {
	return logWithStack()
}

func logWithStack() *logrus.Entry {
	pcs := make([]uintptr, 10)
	size := runtime.Callers(3, pcs)
	stack := make([]string, size)
	for index, pc := range pcs {
		if pc == 0 {
			break
		}
		f := runtime.FuncForPC(pc)
		file, line := f.FileLine(pc)
		//fmt.Printf("--> [%d] %s:%d %s()\n", index, file, line, f.Name())
		stack[index] = fmt.Sprintf("%s:%d %s()", file, line, f.Name())
	}

	fields := logrus.Fields{
		"source": stack[0],
	}

	if os.Getenv("VERBOSE") != "" {
		fields["stackTrace"] = stack
	}
	return logrus.WithFields(fields)
}

func LogOnError(err error) error {
	if err == nil {
		return err
	}

	logWithStack().Error(err)
	return err
}

func PanicOnError(err error) {
	if LogOnError(err) != nil {
		PanicOnError(err)
	}
}
