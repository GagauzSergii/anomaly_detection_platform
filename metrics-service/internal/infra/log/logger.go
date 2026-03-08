package log

import (
	"go.uber.org/zap"
)

// Logger is the thin wrapper of zap.SugaredLogger
// Outer layers depends on this type but not on zap directly
// Realization should be changed if needed

type Logger struct {
	s *zap.SugaredLogger
}

func New() (*Logger, error) {
	// prod config: JSON, layers, stacktrace
	base, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	return &Logger{s: base.Sugar()}, nil
}

type Interface interface {
	Infof(template string, args ...any)
	Errorf(template string, args ...any)
	Infow(msg string, keysAndValues ...any)
	Errorw(msg string, keysAndValues ...any)
	Sync() error
}

func (l *Logger) Infof(template string, args ...any) {
	l.s.Infof(template, args...)
}

func (l *Logger) Errorf(template string, args ...any) {
	l.s.Errorf(template, args...)
}

func (l *Logger) Infow(msg string, keysAndValues ...any) {
	l.s.Infow(msg, keysAndValues...)
}

func (l *Logger) Errorw(msg string, keysAndValues ...any) {
	l.s.Errorw(msg, keysAndValues...)
}

// Sync is needed upon service shutdown (buffer flush)
func (l *Logger) Sync() error {
	return l.s.Sync()
}
