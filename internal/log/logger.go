package log

import (
	"os"

	"github.com/rs/zerolog"
)

type Logger struct {
	zerolog.Logger
}

func NewLogger(debug bool) *Logger {
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	logger := zerolog.New(os.Stdout).Level(level).With().Timestamp().Logger()

	if logger.GetLevel() == zerolog.DebugLevel {
		logger.Info().Msg("debug level enabled")
	}

	return &Logger{logger}
}

// Debugf logs a formatted debugging message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug().Msgf(format, args...)
}

// Infof logs a formatted informational message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info().Msgf(format, args...)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn().Msgf(format, args...)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error().Msgf(format, args...)
}
