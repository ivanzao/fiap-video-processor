package observability

import (
	"io"
	"log/slog"
	"os"
)

func NewLogger(service string) *slog.Logger {
	return NewLoggerTo(os.Stdout, service)
}

func NewLoggerTo(w io.Writer, service string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil)).With("service", service)
}
