/*
Copyright © 2026 Jonathan Herbst <amd64d@gmail.com>
*/
package cmd

import (
	"bufio"
	"errors"
	"fmt"
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
		casc, err := blizzard.OpenCasc(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open casc %s\n", err)
			os.Exit(2)
		}
		defer casc.Close()
		fmt.Printf("casc product - %s, build - %d\n", casc.ProductName, casc.BuildNumber)

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
				if err := cmd.Handler(args, casc); err != nil {
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
		Handler: func(args []string, casc *blizzard.Casc) error {
			for f := range func(yield func(blizzard.FileData) bool) { casc.SearchFiles(args[0], yield) } {
				fmt.Println(f.Name)
			}
			return nil
		},
	})

	register(Command{
		Name: "help",
		Help: "list available commands",
		Handler: func(args []string, casc *blizzard.Casc) error {
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
		Handler: func(args []string, casc *blizzard.Casc) error {
			return ErrExit
		},
	})
	register(Command{
		Name: "quit",
		Help: "alias for exit",
		Handler: func(args []string, casc *blizzard.Casc) error {
			return ErrExit
		},
	})
}

type Command struct {
	Name    string
	Help    string
	Handler func(args []string, casc *blizzard.Casc) error
}

var commands = map[string]Command{}
var ErrExit = errors.New("exit")

func register(c Command) {
	commands[c.Name] = c
}
