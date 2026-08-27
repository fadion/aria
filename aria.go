// Command aria is the Aria language CLI.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fadion/aria/internal/interp"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
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

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "eval, e",
			Usage: "Evaluate `SOURCE` and exit",
		},
	}

	// A bare `aria -e '...'` has no subcommand, so the evaluation runs here.
	// Without a -e, the default help is what an argument-less invocation wants.
	app.Action = func(c *cli.Context) error {
		if src := c.String("eval"); src != "" {
			runSource(source.NewFile("-e", []byte(src)), c.Args())
			return nil
		}
		// An argument left over here is a command that does not exist, and this
		// is where it lands: setting app.Action makes urfave/cli route unmatched
		// arguments to it rather than to CommandNotFound, which is why that hook
		// was dead code and has been removed. Without this, `aria rnu file.ari`
		// printed the help and exited 0, so a typo in a script read as success.
		if c.NArg() > 0 {
			color.Red("Command %q doesn't exist.", c.Args().First())
			os.Exit(2)
		}
		// No arguments at all is a request for help, not a mistake.
		return cli.ShowAppHelp(c)
	}

	// A flag that does not exist is the same kind of mistake as a command that
	// does not exist. By default it printed "Incorrect Usage" and exited 0.
	app.OnUsageError = func(c *cli.Context, err error, isSubcommand bool) error {
		color.Red("%s", err)
		os.Exit(2)
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:      "run",
			Usage:     "Run an Aria source file, or - for standard input",
			ArgsUsage: "FILE",
			Action:    runFile,
		},
		{
			Name:      "check",
			Usage:     "Parse and resolve without running, for CI or an editor",
			ArgsUsage: "FILE",
			Action:    checkFile,
		},
		{
			Name:   "repl",
			Usage:  "Start the interactive repl",
			Action: runREPL,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// readSource reads the first argument as a source file, and returns the rest for
// the program itself. `-` is standard input, so aria can sit in a pipeline.
func readSource(c *cli.Context, command string) (*source.File, []string) {
	args := c.Args()
	if len(args) == 0 {
		// The original printed this and then indexed the empty argument list,
		// panicking with an index-out-of-range.
		color.Red("%s expects a source file.", command)
		os.Exit(2)
	}

	path, rest := args[0], []string(args[1:])
	if path == "-" {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			color.Red("Couldn't read standard input")
			os.Exit(1)
		}
		return source.NewFile("<stdin>", src), rest
	}

	src, err := os.ReadFile(path)
	if err != nil {
		color.Red("Couldn't read '%s'", path)
		os.Exit(1)
	}
	return source.NewFile(path, src), rest
}

func runFile(c *cli.Context) error {
	runSource(readSource(c, "Run"))
	return nil
}

// runSource evaluates a file with the arguments meant for the program.
func runSource(file *source.File, args []string) {
	if !interp.Run(file, interp.Options{
		Out:  os.Stdout,
		Err:  os.Stderr,
		In:   os.Stdin,
		Args: args,
	}) {
		// A failed run exits non-zero, so a script or CI can tell. The original
		// exited 0 on every parse and runtime error alike.
		os.Exit(1)
	}
}

func checkFile(c *cli.Context) error {
	file, _ := readSource(c, "Check")
	if !interp.Check(file, interp.Options{Err: os.Stderr}) {
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
	color.White("Type :help for the commands, or press CTRL+C to leave")
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

	// buffered holds the lines of a construct still being typed. Reading one
	// line at a time meant a multi-line func, module, if or for could not be
	// entered at all, which is most of what a REPL is for.
	var buffered []string

	for {
		prompt := ">> "
		if len(buffered) > 0 {
			prompt = ".. "
		}
		color.Set(color.FgWhite)
		fmt.Print(prompt)
		color.Unset()

		if !input.Scan() {
			fmt.Println()
			return nil
		}
		line := input.Text()

		// An empty line abandons a half-typed construct, which is the way out
		// of one that can never be completed.
		if strings.TrimSpace(line) == "" {
			buffered = nil
			continue
		}

		if len(buffered) == 0 {
			if done, handled := replCommand(session, line); handled {
				if done {
					return nil
				}
				continue
			}
		}

		buffered = append(buffered, line)
		v, err := session.Eval(strings.Join(buffered, "\n"))
		if errors.Is(err, interp.ErrIncomplete) {
			continue
		}
		buffered = nil

		if err != nil {
			// A failed line leaves the session intact, so a typo does not end it.
			color.Red("%s", err)
			continue
		}
		// A statement that produced nothing prints nothing. Half a session used
		// to be `nil` from println calls.
		if v != nil && v != value.NilValue {
			fmt.Println(v.Inspect())
		}
	}
}

// replCommand handles a `:` command, reporting whether it handled the line and
// whether the session should end.
func replCommand(session *interp.Session, line string) (done, handled bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], ":") {
		return false, false
	}

	switch fields[0] {
	case ":help":
		color.White(":help          this list")
		color.White(":load FILE     evaluate a file into this session")
		color.White(":vars          the names declared so far")
		color.White(":modules       the modules in scope")
		color.White(":quit          leave")
	case ":quit", ":exit":
		return true, true
	case ":vars":
		names := session.Declared()
		if len(names) == 0 {
			color.White("nothing declared yet")
			break
		}
		color.White("%s", strings.Join(names, ", "))
	case ":modules":
		color.White("%s", strings.Join(session.Modules(), ", "))
	case ":load":
		if len(fields) != 2 {
			color.Red(":load expects a file")
			break
		}
		src, err := os.ReadFile(fields[1])
		if err != nil {
			color.Red("Couldn't read '%s'", fields[1])
			break
		}
		if _, err := session.Eval(string(src)); err != nil {
			color.Red("%s", err)
		}
	default:
		color.Red("Unknown command %q. Try :help", fields[0])
	}
	return false, true
}
