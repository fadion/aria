// Command aria is the Aria language CLI.
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/fadion/aria/internal/interp"
	"github.com/fadion/aria/internal/source"
	"github.com/fatih/color"
	"github.com/urfave/cli"
)

const version = "0.6.0"

func main() {
	app := cli.NewApp()
	app.Name = "aria"
	app.Usage = "an expressive, noiseless, interpreted toy language"
	app.Authors = []cli.Author{{
		Name:  "Fadion Dashi",
		Email: "jonidashi@gmail.com",
	}}
	app.Version = version

	app.Commands = []cli.Command{
		{
			Name:      "run",
			Usage:     "Run an Aria source file",
			ArgsUsage: "FILE",
			Action:    runFile,
		},
		{
			Name:   "repl",
			Usage:  "Start the interactive repl",
			Action: runREPL,
		},
	}

	app.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Fprintf(ctx.App.Writer, "Command %q doesn't exist.\n", command)
		os.Exit(2)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runFile(c *cli.Context) error {
	args := c.Args()
	if len(args) != 1 {
		// The original printed this and then indexed the empty argument list,
		// panicking with an index-out-of-range.
		color.Red("Run expects exactly one source file.")
		os.Exit(2)
	}

	path := args[0]
	src, err := os.ReadFile(path)
	if err != nil {
		color.Red("Couldn't read '%s'", path)
		os.Exit(1)
	}

	if !interp.Run(source.NewFile(path, src), interp.Options{
		Out: os.Stdout,
		Err: os.Stderr,
		In:  os.Stdin,
	}) {
		// A failed run exits non-zero, so a script or CI can tell. The original
		// exited 0 on every parse and runtime error alike.
		os.Exit(1)
	}
	return nil
}

func runREPL(c *cli.Context) error {
	color.Yellow(`    _   ___ ___   _
   /_\ | _ \_ _| /_\
  / _ \|   /| | / _ \
 /_/ \_\_|_\___/_/ \_\
 `)
	color.White("Close by pressing CTRL+C")
	fmt.Println()

	session, err := interp.NewSession(interp.Options{
		Out: os.Stdout,
		Err: os.Stderr,
		In:  os.Stdin,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	input := bufio.NewScanner(os.Stdin)
	for {
		color.Set(color.FgWhite)
		fmt.Print(">> ")
		color.Unset()

		if !input.Scan() {
			fmt.Println()
			return nil
		}

		line := input.Text()
		if line == "" {
			continue
		}

		v, err := session.Eval(line)
		if err != nil {
			// A failed line leaves the session intact, so a typo does not end it.
			color.Red("%s", err)
			continue
		}
		if v != nil {
			fmt.Println(v.Inspect())
		}
	}
}
