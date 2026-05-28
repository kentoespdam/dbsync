//go:build integration

package engine

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/kentoespdam/dbsync/internal/crypto"
	"github.com/kentoespdam/dbsync/internal/logger"
	"github.com/kentoespdam/dbsync/internal/storage"
)

func TestEngine_ResumeAndDryRun(t *testing.T) {
	ctx := context.Background()

	// 1. Setup MySQL container manually to ensure root access
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForLog("port: 3306  MySQL Community Server"),
	}
	mysqlContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	defer mysqlContainer.Terminate(ctx)

	host, _ := mysqlContainer.Host(ctx)
	port, _ := mysqlContainer.MappedPort(ctx, "3306")
	
	connStr := fmt.Sprintf("root:testpass@tcp(%s:%s)/?multiStatements=true&parseTime=true", host, port.Port())
	db, err := sql.Open("mysql", connStr)
	require.NoError(t, err)
	defer db.Close()

	// Wait for DB to be ready
	require.Eventually(t, func() bool {
		return db.Ping() == nil
	}, 30*time.Second, 1*time.Second)

	// Setup databases and tables
	_, err = db.Exec(`
		CREATE DATABASE srcdb;
		CREATE DATABASE dstdb;
		CREATE TABLE srcdb.users (id INT PRIMARY KEY, name VARCHAR(255));
		CREATE TABLE dstdb.users (id INT PRIMARY KEY, name VARCHAR(255));
	`)
	require.NoError(t, err)

	// Seed source with 200 rows
	for i := 1; i <= 200; i++ {
		_, err = db.Exec("INSERT INTO srcdb.users (id, name) VALUES (?, ?)", i, fmt.Sprintf("user-%d", i))
		require.NoError(t, err)
	}

	// 2. Setup storage
	tmpFile, err := ioutil.TempFile("", "dbsync_engine_test_*.db")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := storage.Open(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	passEnc, _ := crypto.Encrypt([]byte("testpass"), key)

	portInt, _ := strconv.Atoi(port.Port())
	connID, err := store.Connections().Insert(ctx, storage.Connection{
		Name:           "local-test",
		SourceHost:     host,
		SourcePort:     portInt,
		SourceUser:     "root",
		SourcePassword: passEnc,
		SourceDB:       "srcdb",
		DestHost:       host,
		DestPort:       portInt,
		DestUser:       "root",
		DestPassword:   passEnc,
		DestDB:         "dstdb",
	})
	require.NoError(t, err)

	err = store.Mappings().BulkInsert(ctx, []storage.Mapping{
		{ConnectionID: connID, TableName: "users", SourceColumn: sql.NullString{String: "id", Valid: true}, DestColumn: "id"},
		{ConnectionID: connID, TableName: "users", SourceColumn: sql.NullString{String: "name", Valid: true}, DestColumn: "name"},
	})
	require.NoError(t, err)

	l, _ := logger.New("test", "users")
	defer l.Close()

	eng := New(store, key, l)

	t.Run("DryRun", func(t *testing.T) {
		events, err := eng.Run(ctx, Options{
			ConnectionID: connID,
			TableName:    "users",
			BatchSize:    50,
			DryRun:       true,
		})
		require.NoError(t, err)

		var total int
		for e := range events {
			if de, ok := e.(DoneEvent); ok {
				total = de.TotalRows
				assert.NoError(t, de.Err)
			}
		}
		assert.Equal(t, 200, total)

		var count int
		db.QueryRow("SELECT COUNT(*) FROM dstdb.users").Scan(&count)
		assert.Equal(t, 0, count)
	})

	t.Run("PartialSyncAndResume", func(t *testing.T) {
		ctx2, cancel2 := context.WithCancel(ctx)
		events, err := eng.Run(ctx2, Options{
			ConnectionID: connID,
			TableName:    "users",
			BatchSize:    50,
		})
		require.NoError(t, err)

		batchesSeen := 0
		for e := range events {
			if pe, ok := e.(ProgressEvent); ok {
				batchesSeen = pe.Batch
				if batchesSeen == 2 {
					cancel2()
				}
			}
		}

		cp, _ := store.Checkpoints().Get(ctx, connID, "users")
		assert.Equal(t, "interrupted", cp.Status)
		assert.Equal(t, 2, cp.LastBatchCompleted)

		var count int
		db.QueryRow("SELECT COUNT(*) FROM dstdb.users").Scan(&count)
		assert.Equal(t, 100, count)

		// Resume
		events, err = eng.Run(ctx, Options{
			ConnectionID: connID,
			TableName:    "users",
			BatchSize:    50,
		})
		require.NoError(t, err)

		for e := range events {
			if de, ok := e.(DoneEvent); ok {
				assert.NoError(t, de.Err)
				assert.Equal(t, "completed", de.Status)
			}
		}

		db.QueryRow("SELECT COUNT(*) FROM dstdb.users").Scan(&count)
		assert.Equal(t, 200, count)
	})

	t.Run("CompositePK", func(t *testing.T) {
		// Setup composite table
		_, err = db.Exec(`
			CREATE TABLE srcdb.logs (
				app_id INT,
				log_id INT,
				msg TEXT,
				PRIMARY KEY (app_id, log_id)
			);
			CREATE TABLE dstdb.logs (
				app_id INT,
				log_id INT,
				msg TEXT,
				PRIMARY KEY (app_id, log_id)
			);
		`)
		require.NoError(t, err)

		// Seed 10 rows
		for i := 1; i <= 10; i++ {
			_, err = db.Exec("INSERT INTO srcdb.logs (app_id, log_id, msg) VALUES (?, ?, ?)", 1, i, fmt.Sprintf("msg-%d", i))
			require.NoError(t, err)
		}

		err = store.Mappings().BulkInsert(ctx, []storage.Mapping{
			{ConnectionID: connID, TableName: "logs", SourceColumn: sql.NullString{String: "app_id", Valid: true}, DestColumn: "app_id"},
			{ConnectionID: connID, TableName: "logs", SourceColumn: sql.NullString{String: "log_id", Valid: true}, DestColumn: "log_id"},
			{ConnectionID: connID, TableName: "logs", SourceColumn: sql.NullString{String: "msg", Valid: true}, DestColumn: "msg"},
		})
		require.NoError(t, err)

		// Sync with small batch to trigger checkpointing
		events, err := eng.Run(ctx, Options{
			ConnectionID: connID,
			TableName:    "logs",
			BatchSize:    4,
		})
		require.NoError(t, err)

		for e := range events {
			if de, ok := e.(DoneEvent); ok {
				assert.NoError(t, de.Err)
			}
		}

		// Verify row count
		var count int
		db.QueryRow("SELECT COUNT(*) FROM dstdb.logs").Scan(&count)
		assert.Equal(t, 10, count)

		// Verify checkpoint is completed
		cp, _ := store.Checkpoints().Get(ctx, connID, "logs")
		assert.Equal(t, "completed", cp.Status)
	})
}
