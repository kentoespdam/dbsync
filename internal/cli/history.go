package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	historyConnName string
	historyTable    string
	historyLimit    int
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show sync history",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(cmd.Context(), historyConnName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found: %w", historyConnName, err)
		}

		records, err := db.History().List(cmd.Context(), conn.ID, historyTable, historyLimit)
		if err != nil {
			return err
		}

		if len(records) == 0 {
			fmt.Println("No history records found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "STARTED\tDURATION\tTABLE\tROWS\tSTATUS")
		for _, r := range records {
			duration := "-"
			if r.DurationSeconds.Valid {
				duration = fmt.Sprintf("%ds", r.DurationSeconds.Int64)
			}
			rows := "-"
			if r.TotalRows.Valid {
				rows = fmt.Sprintf("%d", r.TotalRows.Int64)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				r.StartedAt.Format("2006-01-02 15:04:05"),
				duration,
				r.TableName,
				rows,
				r.Status,
			)
		}
		w.Flush()

		return nil
	},
}

func init() {
	historyCmd.Flags().StringVar(&historyConnName, "connection", "", "Name of the connection")
	historyCmd.Flags().StringVar(&historyTable, "table", "", "Name of the table")
	historyCmd.Flags().IntVar(&historyLimit, "limit", 10, "Limit number of records")
	historyCmd.MarkFlagRequired("connection")
	historyCmd.MarkFlagRequired("table")
	rootCmd.AddCommand(historyCmd)
}
