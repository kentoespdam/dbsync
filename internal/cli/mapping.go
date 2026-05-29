package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/kentoespdam/dbsync/internal/config"
	"github.com/kentoespdam/dbsync/internal/crypto"
	"github.com/kentoespdam/dbsync/internal/mysql"
	"github.com/kentoespdam/dbsync/internal/storage"
)

var mappingCmd = &cobra.Command{
	Use:   "mapping",
	Short: "Manage column mappings between source and destination",
}

var (
	mappingConnName     string
	mappingTable        string
	mappingYes          bool
	mappingForce        bool
	mappingDest         string
	mappingSource       string
	mappingDefault      string
	mappingValueMap     string // bd-13c
	mappingValueMapFile string // bd-13c
)

var mappingAutoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Automatically generate 1:1 mappings for a table",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(ctx, mappingConnName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found", mappingConnName)
		}

		masterKey, err := config.LoadMasterKey(ctx)
		if err != nil {
			return err
		}

		sPass, err := crypto.Decrypt(conn.SourcePassword, masterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt source password: %v", err)
		}

		dPass, err := crypto.Decrypt(conn.DestPassword, masterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt destination password: %v", err)
		}

		sPool, err := mysql.Open(mysql.Config{
			Host:     conn.SourceHost,
			Port:     conn.SourcePort,
			User:     conn.SourceUser,
			Password: string(sPass),
			DBName:   conn.SourceDB,
		})
		if err != nil {
			return fmt.Errorf("failed to connect to source: %v", err)
		}
		defer sPool.Close()

		dPool, err := mysql.Open(mysql.Config{
			Host:     conn.DestHost,
			Port:     conn.DestPort,
			User:     conn.DestUser,
			Password: string(dPass),
			DBName:   conn.DestDB,
		})
		if err != nil {
			return fmt.Errorf("failed to connect to destination: %v", err)
		}
		defer dPool.Close()

		sCols, err := mysql.DescribeColumns(ctx, sPool.DB(), conn.SourceDB, mappingTable)
		if err != nil {
			return fmt.Errorf("failed to describe source columns: %v", err)
		}

		dCols, err := mysql.DescribeColumns(ctx, dPool.DB(), conn.DestDB, mappingTable)
		if err != nil {
			return fmt.Errorf("failed to describe destination columns: %v", err)
		}

		res := storage.AutoMap(conn.ID, mappingTable, sCols, dCols)

		fmt.Printf("Preview for %s.%s:\n", mappingConnName, mappingTable)
		fmt.Printf("- %d mappings generated\n", len(res.Mappings))

		if len(res.Warnings) > 0 {
			fmt.Printf("\n⚠ %d dest columns are NOT NULL and have no mapping (sync may fail):\n", len(res.Warnings))
			for _, w := range res.Warnings {
				fmt.Printf("  - %s\n", w)
			}
			fmt.Println("\nActionable steps:")
			for _, w := range res.Warnings {
				col := strings.Split(w, " ")[2] // Extract column name from warning
				fmt.Printf("  dbsync mapping set --connection=%s --table=%s --dest=%s --default='VAL'\n", mappingConnName, mappingTable, col)
			}
		}

		if len(res.EnumMismatches) > 0 {
			fmt.Printf("\n⚠ %d dest column has ENUM domain mismatch with source:\n", len(res.EnumMismatches))
			for _, mismatch := range res.EnumMismatches {
				fmt.Printf("  - %s:\n", mismatch.DestColumn)
				fmt.Printf("    source: [%s]\n", strings.Join(mismatch.SourceValues, ", "))
				fmt.Printf("    dest:   [%s]\n", strings.Join(mismatch.DestValues, ", "))
				fmt.Printf("    Run: dbsync mapping set --connection=%s --table=%s --dest=%s --value-map='%s'\n", 
					mappingConnName, mappingTable, mismatch.DestColumn, mismatch.Suggested)
			}
		}

		if len(res.UnmappedSource) > 0 {
			fmt.Printf("\nℹ %d source columns not mapped to destination:\n", len(res.UnmappedSource))
			for _, c := range res.UnmappedSource {
				fmt.Printf("  - %s\n", c)
			}
		}

		exists, _ := db.Mappings().Exists(ctx, conn.ID, mappingTable)
		if exists && !mappingForce {
			fmt.Printf("\nMappings already exist for %s. Overwrite? [y/N]: ", mappingTable)
			if !confirm() {
				fmt.Println("Cancelled")
				return nil
			}
		} else if !mappingYes && !mappingForce {
			fmt.Print("\nApply these mappings? [y/N]: ")
			if !confirm() {
				fmt.Println("Cancelled")
				return nil
			}
		}

		if exists {
			if err := db.Mappings().DeleteByTable(ctx, conn.ID, mappingTable); err != nil {
				return err
			}
		}

		if err := db.Mappings().BulkInsert(ctx, res.Mappings); err != nil {
			return fmt.Errorf("failed to save mappings: %v", err)
		}

		fmt.Println("\nMappings saved successfully")
		return nil
	},
}

var mappingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List mappings for a table",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(ctx, mappingConnName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found", mappingConnName)
		}

		mappings, err := db.Mappings().ListByTable(ctx, conn.ID, mappingTable)
		if err != nil {
			return err
		}

		if len(mappings) == 0 {
			fmt.Printf("No mappings found for %s.%s\n", mappingConnName, mappingTable)
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SOURCE\tDEST\tDEFAULT")
		for _, m := range mappings {
			src := "(NULL)"
			if m.SourceColumn.Valid {
				src = m.SourceColumn.String
			}
			def := "(NULL)"
			if m.DefaultValue.Valid {
				def = m.DefaultValue.String
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", src, m.DestColumn, def)
		}
		w.Flush()

		return nil
	},
}

var mappingSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set or update a specific column mapping",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(ctx, mappingConnName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found", mappingConnName)
		}

		// Fetch existing mapping to preserve unmodified fields on partial update (bd-13c)
		var existing *storage.Mapping
		mappings, err := db.Mappings().ListByTable(ctx, conn.ID, mappingTable)
		if err == nil {
			for _, item := range mappings {
				if item.DestColumn == mappingDest {
					existing = &item
					break
				}
			}
		}

		m := storage.Mapping{
			ConnectionID: conn.ID,
			TableName:    mappingTable,
			DestColumn:   mappingDest,
		}
		if existing != nil {
			m.SourceColumn = existing.SourceColumn
			m.DefaultValue = existing.DefaultValue
			m.ValueMap = existing.ValueMap
		}

		if cmd.Flags().Changed("source") {
			m.SourceColumn = sql.NullString{String: mappingSource, Valid: mappingSource != ""}
		} else if existing == nil {
			m.SourceColumn = sql.NullString{String: mappingSource, Valid: mappingSource != ""}
		}

		if cmd.Flags().Changed("default") {
			m.DefaultValue = sql.NullString{String: mappingDefault, Valid: mappingDefault != ""}
		} else if existing == nil {
			m.DefaultValue = sql.NullString{String: mappingDefault, Valid: mappingDefault != ""}
		}

		var valueMap map[string]string
		if cmd.Flags().Changed("value-map") {
			vmap, err := parseValueMapShorthand(mappingValueMap)
			if err != nil {
				return err
			}
			valueMap = vmap
		} else if cmd.Flags().Changed("value-map-file") {
			vmap, err := parseValueMapFile(mappingValueMapFile)
			if err != nil {
				return err
			}
			valueMap = vmap
		}

		if valueMap != nil {
			if len(valueMap) == 0 {
				m.ValueMap = sql.NullString{Valid: false}
			} else {
				data, err := json.Marshal(valueMap)
				if err != nil {
					return fmt.Errorf("failed to serialize value map: %w", err)
				}
				m.ValueMap = sql.NullString{String: string(data), Valid: true}
			}
		}

		// Validation checks (bd-13c)
		if !m.SourceColumn.Valid && !m.DefaultValue.Valid && !m.ValueMap.Valid {
			return fmt.Errorf("at least one of --source, --default, --value-map, or --value-map-file must be provided")
		}

		if m.ValueMap.Valid && !m.SourceColumn.Valid {
			return fmt.Errorf("value-map requires a valid source column")
		}

		if m.ValueMap.Valid {
			destCol, err := getDestColumn(ctx, conn, mappingTable, mappingDest)
			if err != nil {
				return err
			}
			if err := storage.ValidateMapping(m, destCol); err != nil {
				return err
			}
		}

		if err := db.Mappings().Upsert(ctx, m); err != nil {
			return err
		}

		fmt.Println("Mapping updated successfully")
		return nil
	},
}

var mappingRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove mapping(s) for a table",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		conn, err := db.Connections().GetByName(ctx, mappingConnName)
		if err != nil {
			return fmt.Errorf("connection '%s' not found", mappingConnName)
		}

		if mappingDest == "" {
			fmt.Printf("Remove ALL mappings for %s.%s? [y/N]: ", mappingConnName, mappingTable)
			if !confirm() {
				fmt.Println("Cancelled")
				return nil
			}
			if err := db.Mappings().DeleteByTable(ctx, conn.ID, mappingTable); err != nil {
				return err
			}
			fmt.Println("All mappings removed for table")
		} else {
			// We need to find the ID to delete 1 row, or add DeleteByDest method to repo.
			// Let's add DeleteByDest to MappingRepo for convenience.
			// Actually I can just delete by (conn_id, table, dest).
			// I'll update the repo.
			if err := db.Mappings().DeleteByDest(ctx, conn.ID, mappingTable, mappingDest); err != nil {
				return err
			}
			fmt.Printf("Mapping for %s removed\n", mappingDest)
		}

		return nil
	},
}

func confirm() bool {
	var s string
	fmt.Scanln(&s)
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}

func init() {
	mappingCmd.AddCommand(mappingAutoCmd)
	mappingCmd.AddCommand(mappingListCmd)
	mappingCmd.AddCommand(mappingSetCmd)
	mappingCmd.AddCommand(mappingRmCmd)

	mappingCmd.PersistentFlags().StringVarP(&mappingConnName, "connection", "c", "", "Connection name (required)")
	mappingCmd.MarkPersistentFlagRequired("connection")
	mappingCmd.PersistentFlags().StringVarP(&mappingTable, "table", "t", "", "Table name (required)")
	mappingCmd.MarkPersistentFlagRequired("table")

	mappingAutoCmd.Flags().BoolVarP(&mappingYes, "yes", "y", false, "Skip confirmation prompt")
	mappingAutoCmd.Flags().BoolVarP(&mappingForce, "force", "f", false, "Overwrite existing mappings without asking")

	mappingSetCmd.Flags().StringVar(&mappingDest, "dest", "", "Destination column name (required)")
	mappingSetCmd.MarkFlagRequired("dest")
	mappingSetCmd.Flags().StringVar(&mappingSource, "source", "", "Source column name")
	mappingSetCmd.Flags().StringVar(&mappingDefault, "default", "", "Default value")
	mappingSetCmd.Flags().StringVar(&mappingValueMap, "value-map", "", "Shorthand value mapping (k=v,k=v)") // bd-13c
	mappingSetCmd.Flags().StringVar(&mappingValueMapFile, "value-map-file", "", "JSON file containing value mapping") // bd-13c
	mappingSetCmd.MarkFlagsMutuallyExclusive("value-map", "value-map-file") // bd-13c

	mappingRmCmd.Flags().StringVar(&mappingDest, "dest", "", "Destination column name (optional, if omitted deletes all for table)")
}

// bd-13c
func parseValueMapShorthand(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("value-map shorthand cannot be empty")
	}

	pairs := strings.Split(s, ",")
	result := make(map[string]string)
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		idx := strings.Index(pair, "=")
		if idx == -1 {
			return nil, fmt.Errorf("invalid value-map pair %q (missing '=')", pair)
		}
		k := strings.TrimSpace(pair[:idx])
		v := strings.TrimSpace(pair[idx+1:])
		if k == "" {
			return nil, fmt.Errorf("empty key in pair %q", pair)
		}
		if v == "" {
			return nil, fmt.Errorf("empty value in pair %q", pair)
		}
		if _, exists := result[k]; exists {
			return nil, fmt.Errorf("duplicate key %q in value-map", k)
		}
		result[k] = v
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid value-map pairs found")
	}
	return result, nil
}

// bd-13c
func parseValueMapFile(path string) (map[string]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("value-map-file path cannot be empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read value-map-file: %w", err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse value-map-file: %w", err)
	}

	for k, v := range result {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			return nil, fmt.Errorf("empty key in value-map JSON file")
		}
		if v == "" {
			return nil, fmt.Errorf("empty value for key %q in value-map JSON file", k)
		}
	}

	return result, nil
}

// bd-13c
func getDestColumn(ctx context.Context, conn storage.Connection, tableName, colName string) (mysql.Column, error) {
	masterKey, err := config.LoadMasterKey(ctx)
	if err != nil {
		return mysql.Column{}, fmt.Errorf("failed to load master key: %w", err)
	}

	dPass, err := crypto.Decrypt(conn.DestPassword, masterKey)
	if err != nil {
		return mysql.Column{}, fmt.Errorf("failed to decrypt destination password: %w", err)
	}

	dPool, err := mysql.Open(mysql.Config{
		Host:     conn.DestHost,
		Port:     conn.DestPort,
		User:     conn.DestUser,
		Password: string(dPass),
		DBName:   conn.DestDB,
	})
	if err != nil {
		return mysql.Column{}, fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer dPool.Close()

	dCols, err := mysql.DescribeColumns(ctx, dPool.DB(), conn.DestDB, tableName)
	if err != nil {
		return mysql.Column{}, fmt.Errorf("failed to describe destination columns: %w", err)
	}

	for _, col := range dCols {
		if col.Name == colName {
			return col, nil
		}
	}

	return mysql.Column{}, fmt.Errorf("destination column %q not found in table %q", colName, tableName)
}
