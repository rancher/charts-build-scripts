package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"
)

func Log(ctx context.Context, lvl slog.Level, msg string, attrs ...slog.Attr) {
	logger := slog.Default()
	if !logger.Enabled(ctx, lvl) {
		return
	}
	// Caller information (PC, Func, etc)
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	fs := runtime.CallersFrames(pcs[:])
	f, _ := fs.Next()

	record := slog.NewRecord(time.Now(), lvl, msg, f.PC)

	// Add any attributes passed to the Log function to the record
	record.AddAttrs(attrs...)
	logger.Handler().Handle(ctx, record)
}

// Err - log error with a message
func Err(err error) slog.Attr {
	return slog.Any("error", err)
}

// Fatal - log error and exit with code 1
func Fatal(ctx context.Context, msg string) {
	Log(ctx, slog.LevelError, msg)
	os.Exit(1)
}
