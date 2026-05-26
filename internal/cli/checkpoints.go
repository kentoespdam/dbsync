package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var checkpointsCmd = &cobra.Command{
	Use:   "checkpoints",
	Short: "Manage sync checkpoints",
}

var checkpointsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active checkpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		cps, err := db.Checkpoints().ListActive(cmd.Context())
		if err != nil {
			return err
		}

		if len(cps) == 0 {
			fmt.Println("No active checkpoints found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UPDATED\tCONN_ID\tTABLE\tBATCH\tSTATUS")
		for _, cp := range cps {
			fmt.Fprintf(w, "%s\t%d\t%s\t%d\t%s\n",
				cp.UpdatedAt.Format("2006-01-02 15:04:05"),
				cp.ConnectionID,
				cp.TableName,
				cp.LastBatchCompleted,
				cp.Status,
			)
		}
		w.Flush()

		return nil
	},
}

var (
	resetConnName string
	resetTable    string
)

var checkpointsResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a checkpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(cmd.Context(), resetConnName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found: %w", resetConnName, err)
		}

		err = db.Checkpoints().Delete(cmd.Context(), conn.ID, resetTable)
		if err != nil {
			return err
		}

		fmt.Printf("Checkpoint for %s (table %s) has been reset.\n", resetConnName, resetTable)
		return nil
	},
}

func init() {
	checkpointsResetCmd.Flags().StringVar(&resetConnName, "connection", "", "Name of the connection")
	checkpointsResetCmd.Flags().StringVar(&resetTable, "table", "", "Name of the table")
	checkpointsResetCmd.MarkFlagRequired("connection")
	checkpointsResetCmd.MarkFlagRequired("table")

	checkpointsCmd.AddCommand(checkpointsListCmd)
	checkpointsCmd.AddCommand(checkpointsResetCmd)
	rootCmd.AddCommand(checkpointsCmd)
}
