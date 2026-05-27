package engine

import (
	"encoding/json"
	"fmt"

	"github.com/user/dbsync/internal/logger"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type Engine struct {
	store  *storage.DB
	crypto cryptoDecryptor
	key    []byte
	logger *logger.Logger
}

func New(store *storage.DB, key []byte, log *logger.Logger) *Engine {
	return &Engine{
		store:  store,
		crypto: defaultDecryptor{},
		key:    key,
		logger: log,
	}
}

func serializePK(values []any) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return fmt.Sprint(values[0])
	}
	b, _ := json.Marshal(values)
	return string(b)
}

func deserializePK(s string, pkCols []string, colMap map[string]mysql.Column) ([]any, error) {
	if s == "" {
		return nil, nil
	}
	if len(pkCols) == 1 {
		return []any{s}, nil
	}
	var vals []any
	if err := json.Unmarshal([]byte(s), &vals); err != nil {
		return nil, err
	}
	return vals, nil
}
