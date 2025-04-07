package util

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"
)

func Log(lvl slog.Level, msg string, attrs ...slog.Attr) {
	// internal context because it is a limited CLI app
	ctx := context.Background()
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
	_ = logger.Handler().Handle(ctx, record)
}

func Err(err error) slog.Attr {
	return slog.Any("error", err)
}

// Fatal - log error and exit with code 1
func Fatal(msg string) {
	Log(slog.LevelError, msg)
	os.Exit(1)
}
