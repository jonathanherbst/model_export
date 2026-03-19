/*
Copyright © 2026 Jonathan Herbst <amd64d@gmail.com>
*/
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"jph/model-export/pkg/blizzard"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// cascCmd represents the db2 command
var cascCmd = &cobra.Command{
	Use:   "casc [flags] path",
	Args:  cobra.ExactArgs(1),
	Short: "An interactive tool for interfacing with a blizzard casc",
	Long: `Interactive tool that loads a casc archive and lets you
	- list and search through all the files
	- extract files
	- work with database files`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("opening casc...")
		//casc, err := blizzard.OpenOnlineCasc(args[0], "wow")
		casc, err := blizzard.OpenCasc(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open casc %s\n", err)
			os.Exit(2)
		}
		fmt.Printf("casc product - %s, build - %d\n", casc.ProductName, casc.BuildNumber)

		var casc_extra interface{}
		if blizzard.IsWOWCasc(casc) {
			fmt.Println("detected wow casc, opening...")
			wow, err := blizzard.OpenWOWCasc(casc, ".")
			if err != nil {
				casc.Close()
				fmt.Fprintf(os.Stderr, "Failed to open wow casc %s\n", err)
				os.Exit(2)
			}
			casc_extra = wow
			defer wow.Close()
		} else {
			defer casc.Close()
		}

		listfile, err := cmd.Flags().GetString("listfile")
		if err == nil && listfile != "" {
			casc.ListFilePath = &listfile
		}

		reader := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ")
			if !reader.Scan() {
				// EOF or error
				fmt.Println()
				break
			}
			line := strings.TrimSpace(reader.Text())
			if line == "" {
				continue
			}
			tokens := strings.Fields(line)
			cmdName := tokens[0]
			args := tokens[1:]
			if cmd, ok := commands[cmdName]; ok {
				if err := cmd.Handler(args, casc, casc_extra); err != nil {
					if errors.Is(err, ErrExit) {
						fmt.Println("goodbye")
						break
					}
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmdName)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(cascCmd)
	cascCmd.Flags().String("listfile", "", "Provide a list file for the casc")

	register(Command{
		Name: "ls",
		Help: "ls <match> `List all files that match the match statement`",
		Handler: func(args []string, casc *blizzard.Casc, extra interface{}) error {
			match := ""
			if len(args) > 0 {
				match = args[0]
			}
			for f := range func(yield func(blizzard.FileData) bool) { casc.SearchFiles(match, yield) } {
				fmt.Println(f.Name)
			}
			return nil
		},
	})

	register(Command{
		Name: "x",
		Help: "x <casc_path> <extract_path> `Extract a file from the casc`",
		Handler: func(args []string, casc *blizzard.Casc, extra interface{}) error {
			f, err := os.Create(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open extract file: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()

			casc_f, err := casc.OpenFileByName(args[0], true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open casc file: %v\n", err)
				os.Exit(2)
			}
			defer casc_f.Close()

			len, err := io.Copy(f, casc_f)
			if err != nil {
				return err
			}
			fmt.Printf("%d bytes written to %s\n", len, args[1])
			return nil
		},
	})

	register(Command{
		Name: "tables",
		Help: "tables [<match>] `list tables available in the casc (wow only)`",
		Handler: func(args []string, casc *blizzard.Casc, extra interface{}) error {
			match := ""
			if len(args) > 0 {
				match = args[0]
			}

			switch x := extra.(type) {
			case *blizzard.WOWCasc:
				for name := range x.GetTables {
					if match == "" || strings.Contains(name, match) {
						fmt.Printf("%s ", name)
					}
				}
				fmt.Println()
			default:
				fmt.Println("unsupported casc product")
			}
			return nil
		},
	})

	register(Command{
		Name: "tableinfo",
		Help: "tableinfo <table_name> `print information about table schema`",
		Handler: func(args []string, casc *blizzard.Casc, extra interface{}) error {
			name := args[0]

			switch x := extra.(type) {
			case *blizzard.WOWCasc:
				table, err := x.GetTable(name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to open the table: %v\n", err)
					return nil
				}
				defer table.Close()
				printDBDInfo(*table)
			default:
				fmt.Println("unsupported casc product")
			}
			return nil
		},
	})

	register(Command{
		Name:    "select",
		Help:    "select <columns> from <table_name> `print selected columns from table records`",
		Handler: selectHandler,
	})

	register(Command{
		Name: "help",
		Help: "list available commands",
		Handler: func(args []string, casc *blizzard.Casc, extra interface{}) error {
			fmt.Println("commands:")
			for _, cmd := range commands {
				fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Help)
			}
			return nil
		},
	})

	register(Command{
		Name: "exit",
		Help: "quit the program",
		Handler: func(args []string, casc *blizzard.Casc, extra interface{}) error {
			return ErrExit
		},
	})
	register(Command{
		Name: "quit",
		Help: "alias for exit",
		Handler: func(args []string, casc *blizzard.Casc, extra interface{}) error {
			return ErrExit
		},
	})
}

func selectHandler(args []string, casc *blizzard.Casc, extra interface{}) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: select <columns> from <table_name>\n")
		return nil
	}
	line := strings.Join(args, " ")
	parts := strings.SplitN(line, " from ", 2)
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "usage: select <columns> from <table_name>\n")
		return nil
	}
	columnsStr := strings.TrimSpace(parts[0])
	tableName := strings.TrimSpace(parts[1])
	if columnsStr == "" || tableName == "" {
		fmt.Fprintf(os.Stderr, "usage: select <columns> from <table_name>\n")
		return nil
	}
	columnNames := strings.Split(columnsStr, ",")
	for i, col := range columnNames {
		columnNames[i] = strings.TrimSpace(col)
	}

	switch x := extra.(type) {
	case *blizzard.WOWCasc:
		table, err := x.GetTable(tableName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open the table: %v\n", err)
			return nil
		}
		defer table.Close()

		// Get indices for columns
		var indices []int
		for _, colName := range columnNames {
			idx, err := table.Schema.GetIndexByName(colName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unknown column: %s\n", colName)
				return nil
			}
			indices = append(indices, idx)
		}

		// Iterate records
		for record := range table.GetRecords {
			for i, idx := range indices {
				if i > 0 {
					fmt.Print(" ")
				}
				var field interface{}
				if idx == table.Schema.IDIndex {
					field = uint64(record.GetID())
				} else {
					fieldIndex := idx
					if idx > table.Schema.IDIndex {
						fieldIndex--
					}
					field = record.GetField(fieldIndex)
				}
				switch f := field.(type) {
				case uint64:
					fmt.Printf("%d", f)
				case int64:
					fmt.Printf("%d", f)
				case string:
					fmt.Printf("\"%s\"", f)
				case float32:
					fmt.Printf("%f", f)
				case []uint64:
					fmt.Printf("[%d", f[0])
					for _, v := range f[1:] {
						fmt.Printf(",%d", v)
					}
					fmt.Printf("]")
				case []int64:
					fmt.Printf("[%d", f[0])
					for _, v := range f[1:] {
						fmt.Printf(",%d", v)
					}
					fmt.Printf("]")
				case []float32:
					fmt.Printf("[%f", f[0])
					for _, v := range f[1:] {
						fmt.Printf(",%f", v)
					}
					fmt.Printf("]")
				default:
					panic("unhandled field type")
				}
			}
			fmt.Println()
		}
	default:
		fmt.Println("unsupported casc product")
	}
	return nil
}

type Command struct {
	Name    string
	Help    string
	Handler func(args []string, casc *blizzard.Casc, casc_extra interface{}) error
}

var commands = map[string]Command{}
var ErrExit = errors.New("exit")

func register(c Command) {
	commands[c.Name] = c
}
