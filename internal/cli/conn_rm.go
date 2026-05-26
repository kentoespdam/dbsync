package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var connRmCmd = &cobra.Command{
	Use:   "rm [name]",
	Short: "Remove a database connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(cmd.Context(), name)
		if err != nil {
			return err
		}

		fmt.Printf("Are you sure you want to remove connection '%s'? [y/N]: ", name)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}

		if err := db.Connections().Delete(cmd.Context(), conn.ID); err != nil {
			return err
		}

		fmt.Printf("Connection '%s' removed.\n", name)
		return nil
	},
}
