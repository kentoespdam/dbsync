package cli

import (
	"github.com/spf13/cobra"
)

var connCmd = &cobra.Command{
	Use:   "conn",
	Short: "Manage database connections",
}

func init() {
	connCmd.AddCommand(connAddCmd)
	connCmd.AddCommand(connListCmd)
	connCmd.AddCommand(connRmCmd)
}
