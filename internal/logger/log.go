package logger

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// Logger wraps a logrus.Logger for use across amaros.
type Logger struct {
	*logrus.Logger
}

// New creates a Logger with sensible defaults (text format, info level, stdout).
func New() *Logger {
	l := logrus.New()
	l.SetOutput(os.Stdout)
	l.SetLevel(logrus.InfoLevel)
	l.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return &Logger{Logger: l}
}

// WithField returns a logrus entry with a single field.
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

// WithFields returns a logrus entry with multiple fields.
func (l *Logger) WithFields(fields map[string]interface{}) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// SetLevel sets the log level from a string (e.g. "debug", "info", "warn", "error").
func (l *Logger) SetLevel(level string) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		l.Logger.Warnf("invalid log level %q, defaulting to info", level)
		lvl = logrus.InfoLevel
	}
	l.Logger.SetLevel(lvl)
}

// SetJSON switches the formatter to JSON output.
func (l *Logger) SetJSON() {
	l.Logger.SetFormatter(&logrus.JSONFormatter{})
}

// SetOutput sets the log output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.Logger.SetOutput(w)
}

// ForNode returns a logrus entry tagged with a node name.
func (l *Logger) ForNode(name string) *logrus.Entry {
	return l.Logger.WithField("node", name)
}

// ForComponent returns a logrus entry tagged with a component name.
func (l *Logger) ForComponent(component string) *logrus.Entry {
	return l.Logger.WithField("component", component)
}
