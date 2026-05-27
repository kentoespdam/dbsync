package tui

import "log/slog"

var log = slog.Default().With("pkg", "tui")
