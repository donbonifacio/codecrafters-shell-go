package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Fprint

type CommandArgs struct {
	Cmd  *Command
	Cmds map[string]Command
	Raw  string
	Args []string
	Path []string
	Env  map[string]string
}

type Command struct {
	Name string
	Fn   func(input *CommandArgs) error
}

var commands = map[string]Command{
	"echo": {
		Name: "echo",
		Fn:   echo,
	},
	"exit": {
		Name: "exit",
		Fn:   exit,
	},
	"type": {
		Name: "type",
		Fn:   typeFn,
	},
	"": {
		Name: "unknown command",
		Fn:   unknownCommand,
	},
}

func echo(input *CommandArgs) error {
	arg := strings.Replace(input.Raw, input.Args[0]+" ", "", -1)
	if arg[0] == '$' {
		variable := arg[1:]
		if val, ok := input.Env[variable]; ok {
			fmt.Fprintln(os.Stdout, val)
			return nil
		} else {
			fmt.Fprintln(os.Stdout, "")
			return nil
		}
	}
	fmt.Fprintln(os.Stdout, arg)
	return nil
}

func typeFn(input *CommandArgs) error {
	target := input.Args[1]

	if _, ok := input.Cmds[target]; ok {
		fmt.Fprintf(os.Stdout, "%v is a shell builtin\n", target)
		return nil
	}

	for _, path := range input.Path {
		filename := fmt.Sprintf("%v/%v", path, target)
		if _, err := os.Stat(filename); err == nil {
			fmt.Fprintf(os.Stdout, "%v is %v\n", target, filename)
			return nil
		}
	}

	fmt.Fprintf(os.Stdout, "%v: not found\n", target)
	return nil
}

func exit(input *CommandArgs) error {
	if len(input.Args) > 1 {
		num, err := strconv.Atoi(input.Args[1])
		if err != nil {
			panic(err)
		}
		os.Exit(num)
	}
	os.Exit(0)
	return nil
}

func unknownCommand(input *CommandArgs) error {
	fmt.Fprintf(os.Stdout, "%v: command not found\n", input.Raw)
	return nil
}

func main() {

	for true {
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		raw, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			panic(err)
		}
		raw = strings.TrimSpace(raw)
		input := CommandArgs{
			Cmds: commands,
			Raw:  raw,
			Args: strings.Split(raw, " "),
		}

		path := strings.Split(os.Getenv("PATH"), ":")
		input.Path = path
		input.Env = map[string]string{
			"PATH": os.Getenv("PATH"),
		}

		cmd := commands[input.Args[0]]
		if cmd.Fn != nil {
			input.Cmd = &cmd
			if err := cmd.Fn(&input); err != nil {
				panic(err)
			}
		} else {
			unknownCommand(&input)
		}
	}

}
