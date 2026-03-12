package main

import (
	"bufio"
	"errors"
	"fmt"
	"jph/model-export/pkg/blizzard"
	"os"
	"strings"
)

type Command struct {
	Name    string
	Help    string
	Handler func(args []string) error
}

var commands = map[string]Command{}
var ErrExit = errors.New("exit")

func register(c Command) {
	commands[c.Name] = c
}

func init() {
	register(Command{
		Name: "echo",
		Help: "print the arguments to stdout",
		Handler: func(args []string) error {
			fmt.Println(strings.Join(args, " "))
			return nil
		},
	})

	register(Command{
		Name: "help",
		Help: "list available commands",
		Handler: func(args []string) error {
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
		Handler: func(args []string) error {
			return ErrExit
		},
	})
	register(Command{
		Name: "quit",
		Help: "alias for exit",
		Handler: func(args []string) error {
			return ErrExit
		},
	})
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <path to casc>\n", os.Args[0])
		os.Exit(1)
	}

	casc_path := os.Args[1]
	casc, err := blizzard.OpenCasc(casc_path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open casc %s\n", err)
		os.Exit(2)
	}
	defer casc.Close()
	fmt.Printf("casc product - %s, build - %d\n", casc.ProductName, casc.BuildNumber)

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
			if err := cmd.Handler(args); err != nil {
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
}
