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
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
	"golang.org/x/term"
)

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

var (
	noTest bool
)

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

		fmt.Print("Database: ")
		dDB, _ := reader.ReadString('\n')
		dDB = strings.TrimSpace(dDB)

		if !noTest {
			fmt.Println("\nTesting connections...")
			
			// Test Source
			fmt.Printf("Source (%s:%d/%s)... ", sHost, sPort, sDB)
			sPool, err := mysql.Open(mysql.Config{
				Host:     sHost,
				Port:     sPort,
				User:     sUser,
				Password: string(sPassBytes),
				DBName:   sDB,
			})
			if err != nil {
				fmt.Printf("FAILED: %v\n", err)
				fmt.Print("Continue anyway? [y/N]: ")
				confirm, _ := reader.ReadString('\n')
				if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(confirm)), "y") {
					return fmt.Errorf("connection test failed")
				}
			} else {
				fmt.Println("OK")
				sPool.Close()
			}

			// Test Destination
			fmt.Printf("Destination (%s:%d/%s)... ", dHost, dPort, dDB)
			dPool, err := mysql.Open(mysql.Config{
				Host:     dHost,
				Port:     dPort,
				User:     dUser,
				Password: string(dPassBytes),
				DBName:   dDB,
			})
			if err != nil {
				fmt.Printf("FAILED: %v\n", err)
				fmt.Print("Continue anyway? [y/N]: ")
				confirm, _ := reader.ReadString('\n')
				if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(confirm)), "y") {
					return fmt.Errorf("connection test failed")
				}
			} else {
				fmt.Println("OK")
				dPool.Close()
			}
		}

		sPass, err := crypto.Encrypt(sPassBytes, masterKey)
		if err != nil {
			return err
		}
		dPass, err := crypto.Encrypt(dPassBytes, masterKey)
		if err != nil {
			return err
		}

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

func init() {
	connAddCmd.Flags().BoolVar(&noTest, "no-test", false, "skip connection testing")
}
