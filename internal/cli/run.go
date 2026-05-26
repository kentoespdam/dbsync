package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/user/dbsync/internal/config"
	"github.com/user/dbsync/internal/engine"
	"github.com/user/dbsync/internal/logger"
)

var (
	runConnName string
	runTable    string
	runBatch    int
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a single-batch sync for a table",
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

		l, err := logger.New(runConnName, runTable)
		if err != nil {
			return fmt.Errorf("failed to create logger: %w", err)
		}
		defer l.Close()

		eng := engine.New(db, masterKey, l)
		events, err := eng.Run(ctx, engine.Options{
			ConnectionID: conn.ID,
			TableName:    runTable,
			BatchSize:    runBatch,
		})
		if err != nil {
			return err
		}

		for event := range events {
			switch e := event.(type) {
			case engine.ProgressEvent:
				fmt.Printf("\rBatch %d: %d rows synced", e.Batch, e.RowsDone)
			case engine.BatchErrorEvent:
				fmt.Fprintf(os.Stderr, "\nBatch error: %v\n", e.Err)
			case engine.RowErrorEvent:
				fmt.Fprintf(os.Stderr, "\nRow error (PK=%v): %v\n", e.PK, e.Err)
			case engine.DoneEvent:
				fmt.Printf("\nSync completed. Total rows: %d\n", e.TotalRows)
				if e.Err != nil {
					fmt.Fprintf(os.Stderr, "Fatal error: %v\n", e.Err)
					os.Exit(2)
				}
			}
		}

		return nil
	},
}

func init() {
	runCmd.Flags().StringVar(&runConnName, "connection", "", "Name of the connection to use")
	runCmd.Flags().StringVar(&runTable, "table", "", "Name of the table to sync")
	runCmd.Flags().IntVar(&runBatch, "batch", 1000, "Batch size")
	runCmd.MarkFlagRequired("connection")
	runCmd.MarkFlagRequired("table")
	rootCmd.AddCommand(runCmd)
}
