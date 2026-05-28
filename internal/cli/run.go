package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/kentoespdam/dbsync/internal/config"
	"github.com/kentoespdam/dbsync/internal/engine"
	"github.com/kentoespdam/dbsync/internal/logger"
)

var (
	runConnName  string
	runTable     string
	runAllTables bool
	runBatch     int
	runDryRun    bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run sync for table(s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		// Handle SIGINT
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nAborting...")
			cancel()
		}()

		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(ctx, runConnName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found: %w", runConnName, err)
		}

		masterKey, err := config.LoadMasterKey(ctx)
		if err != nil {
			return err
		}

		var tables []string
		if runAllTables {
			tables, err = db.Mappings().ListDistinctTables(ctx, conn.ID)
			if err != nil {
				return fmt.Errorf("failed to list tables: %w", err)
			}
			if len(tables) == 0 {
				fmt.Println("No mappings found for this connection.")
				return nil
			}
		} else {
			tables = []string{runTable}
		}

		var successCount, partialFailCount, fatalCount int

		for _, table := range tables {
			fmt.Printf("--- Syncing table: %s ---\n", table)
			l, err := logger.New(runConnName, table)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create logger for %s: %v\n", table, err)
				fatalCount++
				continue
			}

			eng := engine.New(db, masterKey, l)
			events, err := eng.Run(ctx, engine.Options{
				ConnectionID: conn.ID,
				TableName:    table,
				BatchSize:    runBatch,
				DryRun:       runDryRun,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start engine for %s: %v\n", table, err)
				l.Close()
				fatalCount++
				continue
			}

			var tableErr error
			for event := range events {
				switch e := event.(type) {
				case engine.ProgressEvent:
					if runDryRun {
						fmt.Printf("\r(DRY RUN) Batch %d: %d rows counted", e.Batch, e.RowsDone)
					} else {
						fmt.Printf("\rBatch %d: %d rows synced", e.Batch, e.RowsDone)
					}
				case engine.BatchErrorEvent:
					fmt.Fprintf(os.Stderr, "\nBatch error: %v\n", e.Err)
				case engine.RowErrorEvent:
					fmt.Fprintf(os.Stderr, "\nRow error (PK=%v): %v\n", e.PK, e.Err)
				case engine.DoneEvent:
					if runDryRun {
						fmt.Printf("\nDry run completed. Total rows estimated: %d\n", e.TotalRows)
					} else {
						fmt.Printf("\nSync %s. Total rows: %d\n", e.Status, e.TotalRows)
					}
					if e.Err != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", e.Err)
						tableErr = e.Err
					}
				}
			}
			l.Close()

			if tableErr != nil {
				if ctx.Err() != nil {
					// Interrupted
					os.Exit(130)
				}
				partialFailCount++
			} else {
				successCount++
			}
			fmt.Println()
		}

		// Final summary and exit code
		if runAllTables {
			fmt.Printf("Summary: %d succeeded, %d failed, %d fatal\n", successCount, partialFailCount, fatalCount)
		}

		if fatalCount > 0 {
			os.Exit(2)
		}
		if partialFailCount > 0 {
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	runCmd.Flags().StringVar(&runConnName, "connection", "", "Name of the connection to use")
	runCmd.Flags().StringVar(&runTable, "table", "", "Name of the table to sync")
	runCmd.Flags().BoolVar(&runAllTables, "all-tables", false, "Sync all tables that have mappings")
	runCmd.Flags().IntVar(&runBatch, "batch", 1000, "Batch size")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Estimate rows without upserting")
	
	runCmd.MarkFlagRequired("connection")
	runCmd.MarkFlagsMutuallyExclusive("table", "all-tables")
	
	rootCmd.AddCommand(runCmd)
}
