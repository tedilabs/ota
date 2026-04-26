package logger

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// Options configure New.
type Options struct {
	// FilePath is the destination for rotated JSON logs. Typical value:
	// ~/.cache/ota/debug.log.
	FilePath string
	// Debug flips the level to slog.LevelDebug.
	Debug bool
	// SessionID is attached to every record as `session_id`.
	SessionID string
	// Sink is an optional override for the write target (tests pass io.Discard).
	Sink io.Writer
}

// New returns a configured *slog.Logger. When Sink is nil and FilePath is
// set, New creates a rotating file at FilePath (10MB × 3, 0600 permissions
// via lumberjack). At least one of Sink or FilePath must be supplied.
func New(opts Options) (*slog.Logger, error) {
	var sink io.Writer = opts.Sink
	if sink == nil {
		if opts.FilePath == "" {
			return nil, errors.New("logger: either Sink or FilePath is required")
		}
		sink = &lumberjack.Logger{
			Filename:   filepath.Clean(opts.FilePath),
			MaxSize:    10, // MB
			MaxBackups: 3,
			Compress:   false,
		}
	}

	level := slog.LevelInfo
	if opts.Debug {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(sink, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: MaskAttr,
	})
	lg := slog.New(handler)
	if opts.SessionID != "" {
		lg = lg.With(slog.String("session_id", opts.SessionID))
	}
	return lg, nil
}
