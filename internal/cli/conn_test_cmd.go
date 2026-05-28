package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/kentoespdam/dbsync/internal/config"
	"github.com/kentoespdam/dbsync/internal/crypto"
	"github.com/kentoespdam/dbsync/internal/mysql"
)

var connTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Test connection to source and destination",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		name := args[0]

		masterKey, err := config.LoadMasterKey(ctx)
		if err != nil {
			return err
		}

		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(ctx, name)
		if err != nil {
			return fmt.Errorf("connection '%s' not found", name)
		}

		// Decrypt passwords
		sPass, err := crypto.Decrypt(conn.SourcePassword, masterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt source password: %v", err)
		}

		dPass, err := crypto.Decrypt(conn.DestPassword, masterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt destination password: %v", err)
		}

		fmt.Printf("Testing connection '%s'...\n", name)

		// Test Source
		fmt.Printf("Source (%s:%d/%s)... ", conn.SourceHost, conn.SourcePort, conn.SourceDB)
		sPool, err := mysql.Open(mysql.Config{
			Host:     conn.SourceHost,
			Port:     conn.SourcePort,
			User:     conn.SourceUser,
			Password: string(sPass),
			DBName:   conn.SourceDB,
		})
		if err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("source connection failed: %v", err)
		}
		sPool.Close()
		fmt.Println("OK")

		// Test Destination
		fmt.Printf("Destination (%s:%d/%s)... ", conn.DestHost, conn.DestPort, conn.DestDB)
		dPool, err := mysql.Open(mysql.Config{
			Host:     conn.DestHost,
			Port:     conn.DestPort,
			User:     conn.DestUser,
			Password: string(dPass),
			DBName:   conn.DestDB,
		})
		if err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("destination connection failed: %v", err)
		}
		dPool.Close()
		fmt.Println("OK")

		return nil
	},
}
