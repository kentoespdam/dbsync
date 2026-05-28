package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/kentoespdam/dbsync/internal/config"
	"github.com/kentoespdam/dbsync/internal/storage"
)

var (
	dbPath string
)

var rootCmd = &cobra.Command{
	Use:   "dbsync",
	Short: "dbsync is a tool to sync MySQL databases",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			if wiped, err := config.EnsureConsistentState(); err != nil {
				return fmt.Errorf("reconcile portable state: %v", err)
			} else if wiped {
				fmt.Fprintln(os.Stderr, "Notice: salt/db pair was inconsistent; wiped orphan so first-run setup can proceed.")
			}

			var err error
			dbPath, err = config.DBPath()
			if err != nil {
				return err
			}
		}

		if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
			return fmt.Errorf("failed to create data directory: %v", err)
		}

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "path to the SQLite database")
	rootCmd.AddCommand(connCmd)
	rootCmd.AddCommand(tablesCmd)
	rootCmd.AddCommand(mappingCmd)
}

func getDB() (*storage.DB, error) {
	return storage.Open(dbPath)
}
