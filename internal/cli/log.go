package cli

import "log/slog"

var log = slog.Default().With("pkg", "cli")
