package applog

import (
	"context"
	"log/slog"

	"github.com/user/dbsync/internal/redact"
)

type redactHandler struct {
	slog.Handler
}

func (h *redactHandler) Handle(ctx context.Context, r slog.Record) error {
	var attrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, h.redactAttr(a))
		return true
	})

	newR := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	newR.AddAttrs(attrs...)

	return h.Handler.Handle(ctx, newR)
}

func (h *redactHandler) redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		groupAttrs := a.Value.Group()
		newGroupAttrs := make([]slog.Attr, len(groupAttrs))
		for i, ga := range groupAttrs {
			newGroupAttrs[i] = h.redactAttr(ga)
		}
		// Convert []slog.Attr to []any for slog.Group
		args := make([]any, len(newGroupAttrs))
		for i, v := range newGroupAttrs {
			args[i] = v
		}
		return slog.Group(a.Key, args...)
	}

	if a.Key == "err" || a.Key == "error" {
		if err, ok := a.Value.Any().(error); ok {
			return slog.String(a.Key, redact.Error(err))
		}
	}
	return a
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &redactHandler{h.Handler.WithAttrs(redacted)}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{h.Handler.WithGroup(name)}
}
