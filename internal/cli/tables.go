package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/user/dbsync/internal/config"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/mysql"
)

var (
	connName string
)

var tablesCmd = &cobra.Command{
	Use:   "tables",
	Short: "Inspect database tables",
}

var tablesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tables and their primary keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if connName == "" {
			return fmt.Errorf("connection name is required (--connection)")
		}

		masterKey, err := config.LoadMasterKey(ctx)
		if err != nil {
			return err
		}

		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(ctx, connName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found", connName)
		}

		// Decrypt source password
		sPass, err := crypto.Decrypt(conn.SourcePassword, masterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt source password: %v", err)
		}

		// Connect to source
		pool, err := mysql.Open(mysql.Config{
			Host:     conn.SourceHost,
			Port:     conn.SourcePort,
			User:     conn.SourceUser,
			Password: string(sPass),
			DBName:   conn.SourceDB,
		})
		if err != nil {
			return err
		}
		defer pool.Close()

		tables, err := mysql.ListTables(ctx, pool.DB(), conn.SourceDB)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TABLE\tPRIMARY KEY")
		fmt.Fprintln(w, "-----\t-----------")

		for _, table := range tables {
			pks, err := mysql.DetectPK(ctx, pool.DB(), conn.SourceDB, table)
			if err != nil {
				return fmt.Errorf("failed to detect PK for %s: %v", table, err)
			}

			pkStr := strings.Join(pks, ", ")
			if pkStr == "" {
				pkStr = "(no PK)"
			}
			fmt.Fprintf(w, "%s\t%s\n", table, pkStr)
		}
		w.Flush()

		return nil
	},
}

func init() {
	tablesListCmd.Flags().StringVarP(&connName, "connection", "c", "", "connection name (required)")
	tablesListCmd.MarkFlagRequired("connection")
	tablesCmd.AddCommand(tablesListCmd)
}
