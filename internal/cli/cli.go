package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/dbsync/internal/storage"
)

var (
	dbPath string
)

var rootCmd = &cobra.Command{
	Use:   "dbsync",
	Short: "dbsync is a tool to sync MySQL databases",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			var err error
			dbPath, err = defaultDBPath()
			if err != nil {
				return err
			}
		}
		
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
			return fmt.Errorf("failed to create data directory: %v", err)
		}
		
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "path to the SQLite database")
	rootCmd.AddCommand(connCmd)
}

func defaultDBPath() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "dbsync", "dbsync.db"), nil
}

func getDB() (*storage.DB, error) {
	return storage.Open(dbPath)
}
