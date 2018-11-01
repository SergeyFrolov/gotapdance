package tapdance

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"sync"
)

// implements interface logrus.Formatter
type formatter struct {
}

func (f *formatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("[%s] %s\n", entry.Time.Format("15:04:05"), entry.Message)), nil
}

var logrusLogger *logrus.Logger
var initLoggerOnce sync.Once

// Logger is an access point for TapDance-wide logger
func Logger() *logrus.Logger {
	initLoggerOnce.Do(func() {
		logrusLogger = logrus.New()
		logrusLogger.Formatter = new(formatter)
		logrusLogger.Level = logrus.InfoLevel
		// logrusLogger.Level = logrus.DebugLevel

		// CI will overwrite the build string with branch and commit hash
		const build string = "[BUILD]"
		if build != "[BUILD]" {
			logrusLogger.Infof("Running gotapdance build %s", build)
		}
	})
	return logrusLogger
}
