package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/dbsync/internal/config"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/storage"
	"golang.org/x/term"
)

var connCmd = &cobra.Command{
	Use:   "conn",
	Short: "Manage database connections",
}

func readPassword(reader *bufio.Reader, prompt string) ([]byte, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		p, err := term.ReadPassword(fd)
		fmt.Println()
		return p, err
	}
	
	p, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(p)), nil
}

var connAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new database connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		
		masterKey, err := config.LoadMasterKey(ctx)
		if err != nil {
			return err
		}

		db, err := getDB()
		if err != nil {
			return err
		}
		defer db.Close()

		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Connection name: ")
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)

		fmt.Println("\nSource MySQL (Extraction Source)")
		fmt.Print("Host [localhost]: ")
		sHost, _ := reader.ReadString('\n')
		sHost = strings.TrimSpace(sHost)
		if sHost == "" {
			sHost = "localhost"
		}

		fmt.Print("Port [3306]: ")
		sPortStr, _ := reader.ReadString('\n')
		sPortStr = strings.TrimSpace(sPortStr)
		sPort := 3306
		if sPortStr != "" {
			sPort, _ = strconv.Atoi(sPortStr)
		}

		fmt.Print("User: ")
		sUser, _ := reader.ReadString('\n')
		sUser = strings.TrimSpace(sUser)

		sPassBytes, err := readPassword(reader, "Password: ")
		if err != nil {
			return err
		}
		sPass, err := crypto.Encrypt(sPassBytes, masterKey)
		if err != nil {
			return err
		}

		fmt.Print("Database: ")
		sDB, _ := reader.ReadString('\n')
		sDB = strings.TrimSpace(sDB)

		fmt.Println("\nDestination MySQL (Loading Target)")
		fmt.Print("Host: ")
		dHost, _ := reader.ReadString('\n')
		dHost = strings.TrimSpace(dHost)

		fmt.Print("Port [3306]: ")
		dPortStr, _ := reader.ReadString('\n')
		dPortStr = strings.TrimSpace(dPortStr)
		dPort := 3306
		if dPortStr != "" {
			dPort, _ = strconv.Atoi(dPortStr)
		}

		fmt.Print("User: ")
		dUser, _ := reader.ReadString('\n')
		dUser = strings.TrimSpace(dUser)

		dPassBytes, err := readPassword(reader, "Password: ")
		if err != nil {
			return err
		}
		dPass, err := crypto.Encrypt(dPassBytes, masterKey)
		if err != nil {
			return err
		}

		fmt.Print("Database: ")
		dDB, _ := reader.ReadString('\n')
		dDB = strings.TrimSpace(dDB)

		conn := storage.Connection{
			Name:           name,
			SourceHost:     sHost,
			SourcePort:     sPort,
			SourceUser:     sUser,
			SourcePassword: sPass,
			SourceDB:       sDB,
			DestHost:       dHost,
			DestPort:       dPort,
			DestUser:       dUser,
			DestPassword:   dPass,
			DestDB:         dDB,
		}

		id, err := db.Connections().Insert(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to save connection: %v", err)
		}

		fmt.Printf("\nConnection '%s' added successfully (ID: %d)\n", name, id)
		return nil
	},
}

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

func init() {
	connCmd.AddCommand(connAddCmd)
	connCmd.AddCommand(connListCmd)
	connCmd.AddCommand(connRmCmd)
}
