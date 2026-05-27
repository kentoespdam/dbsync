//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mysqlmodule "github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestMySQLOps(t *testing.T) {
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
		CREATE TABLE items (
			id INT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE composite_items (
			shop_id INT,
			item_id INT,
			price DECIMAL(10,2),
			PRIMARY KEY (shop_id, item_id)
		)
	`)
	require.NoError(t, err)

	t.Run("SelectBatch", func(t *testing.T) {
		// Seed data
		for i := 1; i <= 5; i++ {
			_, err = db.Exec("INSERT INTO items (id, name) VALUES (?, ?)", i, fmt.Sprintf("item-%d", i))
			require.NoError(t, err)
		}

		t.Run("FirstBatch", func(t *testing.T) {
			rows, nextPK, err := SelectBatch(ctx, db, "testdb", "items", nil, []string{"id"}, nil, 3)
			assert.NoError(t, err)
			assert.Len(t, rows, 3)
			assert.Equal(t, int64(1), rows[0]["id"])
			assert.Equal(t, int64(3), rows[2]["id"])
			assert.Equal(t, []any{int64(3)}, nextPK)
		})

		t.Run("SecondBatch", func(t *testing.T) {
			rows, nextPK, err := SelectBatch(ctx, db, "testdb", "items", nil, []string{"id"}, []any{int64(3)}, 3)
			assert.NoError(t, err)
			assert.Len(t, rows, 2)
			assert.Equal(t, int64(4), rows[0]["id"])
			assert.Equal(t, int64(5), rows[1]["id"])
			assert.Equal(t, []any{int64(5)}, nextPK)
		})

		t.Run("SpecificCols", func(t *testing.T) {
			rows, _, err := SelectBatch(ctx, db, "testdb", "items", []string{"id", "name"}, []string{"id"}, nil, 1)
			assert.NoError(t, err)
			assert.Len(t, rows, 1)
			assert.Contains(t, rows[0], "id")
			assert.Contains(t, rows[0], "name")
			assert.NotContains(t, rows[0], "description")
		})

		t.Run("CompositePK", func(t *testing.T) {
			// Seed composite data
			for s := 1; s <= 2; s++ {
				for i := 1; i <= 3; i++ {
					_, err = db.Exec("INSERT INTO composite_items (shop_id, item_id, price) VALUES (?, ?, ?)", s, i, float64(s*10+i))
					require.NoError(t, err)
				}
			}

			// First batch
			rows, nextPK, err := SelectBatch(ctx, db, "testdb", "composite_items", nil, []string{"shop_id", "item_id"}, nil, 2)
			assert.NoError(t, err)
			assert.Len(t, rows, 2)
			assert.Equal(t, int64(1), rows[0]["shop_id"])
			assert.Equal(t, int64(1), rows[0]["item_id"])
			assert.Equal(t, int64(1), rows[1]["shop_id"])
			assert.Equal(t, int64(2), rows[1]["item_id"])
			assert.Equal(t, []any{int64(1), int64(2)}, nextPK)

			// Next batch from (1, 2)
			rows, nextPK, err = SelectBatch(ctx, db, "testdb", "composite_items", nil, []string{"shop_id", "item_id"}, nextPK, 2)
			assert.NoError(t, err)
			assert.Len(t, rows, 2)
			assert.Equal(t, int64(1), rows[0]["shop_id"])
			assert.Equal(t, int64(3), rows[0]["item_id"])
			assert.Equal(t, int64(2), rows[1]["shop_id"])
			assert.Equal(t, int64(1), rows[1]["item_id"])
		})
	})

	t.Run("Upsert", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		mappings := []ResolvedMapping{
			{DestColumn: "id", ValueFn: func(row Row) (any, error) { return row["id"], nil }},
			{DestColumn: "name", ValueFn: func(row Row) (any, error) { return row["name"], nil }},
			{DestColumn: "description", ValueFn: func(row Row) (any, error) { return "upserted", nil }},
		}

		rows := []Row{
			{"id": 1, "name": "item-1-updated"}, // existing
			{"id": 10, "name": "item-10-new"},   // new
		}

		count, err := Upsert(ctx, tx, "testdb", "items", []string{"id"}, mappings, rows)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)

		require.NoError(t, tx.Commit())

		// Verify updates
		var name, desc string
		err = db.QueryRow("SELECT name, description FROM items WHERE id = 1").Scan(&name, &desc)
		assert.NoError(t, err)
		assert.Equal(t, "item-1-updated", name)
		assert.Equal(t, "upserted", desc)

		err = db.QueryRow("SELECT name, description FROM items WHERE id = 10").Scan(&name, &desc)
		assert.NoError(t, err)
		assert.Equal(t, "item-10-new", name)
		assert.Equal(t, "upserted", desc)
	})
}
