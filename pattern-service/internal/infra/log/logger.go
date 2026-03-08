package log

import (
	"go.uber.org/zap"
)

type Interface interface {
	Infow(msg string, keysAndValues ...any)
	Errorw(msg string, keysAndValues ...any)
	Sync() error
}

type Logger struct {
	logger *zap.SugaredLogger
}

func New() (*Logger, error) {
	cfg := zap.NewDevelopmentConfig()
	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return &Logger{logger: logger.Sugar()}, nil
}

func (l *Logger) Infow(msg string, keysAndValues ...any) {
	l.logger.Infow(msg, keysAndValues...)
}

func (l *Logger) Errorw(msg string, keysAndValues ...any) {
	l.logger.Errorw(msg, keysAndValues...)
}

func (l *Logger) Sync() error {
	return l.logger.Sync()
}
