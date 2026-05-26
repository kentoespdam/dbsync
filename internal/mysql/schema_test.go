//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mysqlmodule "github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestMySQLSchema(t *testing.T) {
	ctx := context.Background()

	// Setup MySQL container
	mysqlContainer, err := mysqlmodule.Run(ctx,
		"mysql:8.0",
		mysqlmodule.WithDatabase("testdb"),
		mysqlmodule.WithUsername("testuser"),
		mysqlmodule.WithPassword("testpass"),
	)
	require.NoError(t, err)
	defer mysqlContainer.Terminate(ctx)

	connStr, err := mysqlContainer.ConnectionString(ctx, "parseTime=true")
	require.NoError(t, err)

	db, err := sql.Open("mysql", connStr)
	require.NoError(t, err)
	defer db.Close()

	// Wait for DB to be ready
	require.Eventually(t, func() bool {
		return db.Ping() == nil
	}, 30*time.Second, 1*time.Second)

	// Setup test tables
	_, err = db.Exec(`
		CREATE TABLE users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			email VARCHAR(255) NULL,
			UNIQUE(username)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE composite_test (
			tenant_id INT,
			user_id INT,
			data TEXT,
			PRIMARY KEY (tenant_id, user_id)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE no_pk (
			name VARCHAR(255)
		)
	`)
	require.NoError(t, err)

	t.Run("ListTables", func(t *testing.T) {
		tables, err := ListTables(ctx, db, "testdb")
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"users", "composite_test", "no_pk"}, tables)
	})

	t.Run("DetectPK", func(t *testing.T) {
		t.Run("SingleColumn", func(t *testing.T) {
			pks, err := DetectPK(ctx, db, "testdb", "users")
			assert.NoError(t, err)
			assert.Equal(t, []string{"id"}, pks)
		})

		t.Run("Composite", func(t *testing.T) {
			pks, err := DetectPK(ctx, db, "testdb", "composite_test")
			assert.NoError(t, err)
			assert.Equal(t, []string{"tenant_id", "user_id"}, pks)
		})

		t.Run("None", func(t *testing.T) {
			pks, err := DetectPK(ctx, db, "testdb", "no_pk")
			assert.NoError(t, err)
			assert.Empty(t, pks)
		})
	})

	t.Run("DescribeColumns", func(t *testing.T) {
		cols, err := DescribeColumns(ctx, db, "testdb", "users")
		assert.NoError(t, err)
		require.Len(t, cols, 3)

		assert.Equal(t, "id", cols[0].Name)
		assert.Equal(t, "int", cols[0].DataType)
		assert.False(t, cols[0].IsNullable)
		assert.Equal(t, "PRI", cols[0].ColumnKey)

		assert.Equal(t, "username", cols[1].Name)
		assert.False(t, cols[1].IsNullable)
		assert.Equal(t, "UNI", cols[1].ColumnKey)

		assert.Equal(t, "email", cols[2].Name)
		assert.True(t, cols[2].IsNullable)
	})
}
