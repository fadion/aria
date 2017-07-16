package main

import (
	"fmt"
	"os"
	"bufio"
	"io/ioutil"
	"github.com/fatih/color"
	"github.com/urfave/cli"
	"github.com/fadion/aria/lexer"
	"github.com/fadion/aria/reader"
	"github.com/fadion/aria/reporter"
	"github.com/fadion/aria/parser"
	"github.com/fadion/aria/interpreter"
)

func main() {
	app := cli.NewApp()
	app.Name = "aria"
	app.Usage = "an expressive, noiseless, interpreted toy language"
	app.Authors = []cli.Author{{
		Name:  "Fadion Dashi",
		Email: "jonidashi@gmail.com",
	}}
	app.Version = "0.1.0"

	app.Commands = []cli.Command{
		{
			Name:  "run",
			Usage: "Run an Aria source file",
			Action: func(c *cli.Context) error {
				if len(c.Args()) != 1 {
					color.Red("Run expects a source file as argument.")
				}

				file := c.Args()[0]
				source, err := ioutil.ReadFile(file)
				if err != nil {
					color.Red("Couldn't read '%s'", file)
					return nil
				}

				lex := lexer.New(reader.New(source))
				if reporter.HasErrors() {
					printErrors()
					return nil
				}

				parse := parser.New(lex)
				program := parse.Parse()
				if reporter.HasErrors() {
					printErrors()
					return nil
				}

				runner := interpreter.New()
				runner.Interpret(program, interpreter.NewScope())
				if reporter.HasErrors() {
					printErrors()
					return nil
				}

				return nil
			},
		},
		{
			Name:  "repl",
			Usage: "Start the interactive repl",
			Action: func(c *cli.Context) error {
				input := bufio.NewReader(os.Stdin)
				color.Yellow(`    _   ___ ___   _
   /_\ | _ \_ _| /_\
  / _ \|   /| | / _ \
 /_/ \_\_|_\___/_/ \_\
 `)
				color.White("Close by pressing CTRL+C")
				fmt.Println()

				scope := interpreter.NewScope()

				for {
					color.Set(color.FgWhite)
					fmt.Print(">> ")
					color.Unset()

					source, _ := input.ReadBytes('\n')
					lex := lexer.New(reader.New(source))
					if reporter.HasErrors() {
						printErrors()
						continue
					}

					parse := parser.New(lex)
					program := parse.Parse()
					if reporter.HasErrors() {
						printErrors()
						continue
					}

					runner := interpreter.New()
					object := runner.Interpret(program, scope)
					if reporter.HasErrors() {
						printErrors()
						continue
					}

					if object != nil {
						fmt.Println(object.Inspect())
					}
				}

				return nil
			},
		},
	}

	app.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Fprintf(ctx.App.Writer, "Command %q doesn't exist.\n", command)
	}

	app.Run(os.Args)
}

func printErrors() {
	color.White("Oops, found some errors:")
	for _, v := range reporter.GetErrors() {
		color.Red(v)
	}
	reporter.ClearErrors()
}