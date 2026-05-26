package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var connListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all database connections",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		connections, err := db.Connections().List(cmd.Context())
		if err != nil {
			return err
		}

		fmt.Printf("%-20s | %-30s | %-30s | %-20s\n", "NAME", "SOURCE", "DEST", "CREATED")
		fmt.Println(strings.Repeat("-", 105))

		for _, c := range connections {
			source := fmt.Sprintf("%s@%s:%d/%s", c.SourceUser, c.SourceHost, c.SourcePort, c.SourceDB)
			dest := fmt.Sprintf("%s@%s:%d/%s", c.DestUser, c.DestHost, c.DestPort, c.DestDB)
			fmt.Printf("%-20s | %-30s | %-30s | %-20s\n", 
				c.Name, source, dest, c.CreatedAt.Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}
