package logger

import "log/slog"

// SlogLogger wraps the standard library slog as the default Logger.
type SlogLogger struct {
	log *slog.Logger
}

// NewSlog creates a logger backed by slog.
func NewSlog() *SlogLogger {
	return &SlogLogger{log: slog.Default()}
}

func (s *SlogLogger) Info(msg string, args ...any)  { s.log.Info(msg, args...) }
func (s *SlogLogger) Error(msg string, args ...any) { s.log.Error(msg, args...) }
func (s *SlogLogger) Debug(msg string, args ...any) { s.log.Debug(msg, args...) }
