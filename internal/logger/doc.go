// Package logger constructs a configured slog.Logger with a session_id
// correlation field and a file-sink (lumberjack) rotated at 10MB × 3
// (REQ-O01 AC-3). A masking ReplaceAttr scrubs sensitive keys before write.
package logger
