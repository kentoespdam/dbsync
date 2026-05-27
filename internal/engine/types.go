package engine

import (
	"github.com/user/dbsync/internal/crypto"
)

type cryptoDecryptor interface {
	Decrypt(b64 string, key []byte) ([]byte, error)
}

type defaultDecryptor struct{}

func (d defaultDecryptor) Decrypt(b64 string, key []byte) ([]byte, error) {
	return crypto.Decrypt(b64, key) // circular dep? No, engine imports crypto.
}

type Options struct {
	ConnectionID int64
	TableName    string
	BatchSize    int // default 1000
	DryRun       bool
}

type Event interface{ isEvent() }

type ProgressEvent struct {
	Batch    int
	RowsDone int
}

func (e ProgressEvent) isEvent() {}

type BatchErrorEvent struct {
	Batch int
	Err   error
}

func (e BatchErrorEvent) isEvent() {}

type RowErrorEvent struct {
	Batch int
	PK    any
	Err   error
}

func (e RowErrorEvent) isEvent() {}

type DoneEvent struct {
	TotalRows int
	Status    string // 'completed', 'failed', 'interrupted'
	Err       error  // nil if success
}

func (e DoneEvent) isEvent() {}
