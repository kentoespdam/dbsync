package engine

import "log/slog"

var log = slog.Default().With("pkg", "engine")
