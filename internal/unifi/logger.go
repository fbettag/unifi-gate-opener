package unifi

import "github.com/sirupsen/logrus"

// Logger interface for UniFi client
type Logger interface {
	Debugf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
}

// LogrusAdapter adapts logrus.Logger to our Logger interface
type LogrusAdapter struct {
	logger *logrus.Logger
}

// NewLogrusAdapter creates a logger adapter for logrus
func NewLogrusAdapter(logger *logrus.Logger) *LogrusAdapter {
	return &LogrusAdapter{logger: logger}
}

func (la *LogrusAdapter) Debugf(format string, args ...interface{}) {
	la.logger.Debugf(format, args...)
}

func (la *LogrusAdapter) Errorf(format string, args ...interface{}) {
	la.logger.Errorf(format, args...)
}

func (la *LogrusAdapter) Infof(format string, args ...interface{}) {
	la.logger.Infof(format, args...)
}

// TestLogger implements Logger interface using testing.T
type TestLogger struct {
	t interface {
		Logf(format string, args ...interface{})
	}
}

// NewTestLogger creates a logger that uses testing.T
func NewTestLogger(t interface {
	Logf(format string, args ...interface{})
}) *TestLogger {
	return &TestLogger{t: t}
}

func (tl *TestLogger) Debugf(format string, args ...interface{}) {
	tl.t.Logf("[DEBUG] "+format, args...)
}

func (tl *TestLogger) Errorf(format string, args ...interface{}) {
	tl.t.Logf("[ERROR] "+format, args...)
}

func (tl *TestLogger) Infof(format string, args ...interface{}) {
	tl.t.Logf("[INFO] "+format, args...)
}
