package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Fprint

type CommandArgs struct {
	Cmd         *Command
	Cmds        map[string]Command
	Raw         string
	Args        []string
	Path        []string
	Env         map[string]string
	Exe         string
	StartingDir string
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
	"exec": {
		Name: "exec",
		Fn:   execute,
	},
	"pwd": {
		Name: "pwd",
		Fn:   pwd,
	},
	"cd": {
		Name: "cd",
		Fn:   changeDirectory,
	},
	"": {
		Name: "unknown command",
		Fn:   unknownCommand,
	},
}

func processPath(input *CommandArgs, path string) (string, error) {
	newPath := path

	if strings.HasPrefix(newPath, "/") {
		return newPath, nil
	}

	if strings.HasPrefix(newPath, "~") {
		return strings.Replace(newPath, "~", input.Env["HOME"], -1), nil
	}

	if strings.HasPrefix(newPath, "./") {
		return strings.Replace(newPath, ".", input.Env["PWD"], -1), nil
	}

	pwd := input.Env["PWD"]
	if strings.HasSuffix(pwd, "/") {
		pwd = pwd[:len(pwd)-1]
	}
	//fmt.Printf("pwd: %v\n", pwd)
	for strings.HasPrefix(newPath, "..") {
		if pwd == "" { // we want to go back, but nothing to go back to
			return path, fmt.Errorf("invalid path")
		}
		parts := strings.Split(pwd, "/")
		pwd = strings.Join(parts[:len(parts)-1], "/")
		tentativePath := strings.Replace(newPath, "../", "", 1)
		if tentativePath == newPath {
			newPath = strings.Replace(newPath, "..", "", 1)
		} else {
			newPath = tentativePath
		}
		//fmt.Printf("pwd: %v newPath: %v\n", pwd, newPath)
	}
	return strings.TrimSuffix(fmt.Sprintf("%v/%v", pwd, newPath), "/"), nil
}

func changeDirectory(input *CommandArgs) error {
	if len(input.Args) <= 1 {
		return nil
	}

	newDir, err := processPath(input, input.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stdout, "cd: %v: No such file or directory\n", newDir)
		return nil
	}

	if _, err := os.Stat(newDir); err != nil {
		fmt.Fprintf(os.Stdout, "cd: %v: No such file or directory\n", newDir)
		return nil
	}
	os.Chdir(newDir)
	input.Env["PWD"] = newDir
	return nil
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
	target := input.Args[0]
	if target == "type" {
		target = input.Args[1]
	}

	if _, ok := input.Cmds[target]; ok {
		fmt.Fprintf(os.Stdout, "%v is a shell builtin\n", target)
		return nil
	}

	for _, path := range input.Path {
		filename := fmt.Sprintf("%v/%v", path, target)
		if _, err := os.Stat(filename); err == nil {
			input.Exe = filename
			if input.Args[0] == "type" {
				fmt.Fprintf(os.Stdout, "%v is %v\n", target, filename)
			}
			return nil
		}
	}

	fmt.Fprintf(os.Stdout, "%v: not found\n", target)
	return nil
}

func pwd(input *CommandArgs) error {
	fmt.Fprintf(os.Stdout, "%v\n", input.Env["PWD"])
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

func execute(input *CommandArgs) error {
	exe := input.Exe
	cmd := exec.Command(exe)
	if len(input.Args) > 1 {
		cmd.Args = input.Args
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func unknownCommand(input *CommandArgs) error {
	fmt.Fprintf(os.Stdout, "%v: command not found\n", input.Raw)
	return nil
}

func main() {
	env := map[string]string{}

	path := strings.Split(os.Getenv("PATH"), ":")
	rawPath := os.Getenv("PATH")
	env["PATH"] = rawPath

	startingDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	env["PWD"] = startingDir

	env["HOME"] = os.Getenv("HOME")

	for true {
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		raw, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			panic(err)
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		input := CommandArgs{
			Cmds:        commands,
			Raw:         raw,
			Args:        strings.Split(raw, " "),
			Path:        path,
			StartingDir: startingDir,
			Env:         env,
		}

		cmd := commands[input.Args[0]]
		if cmd.Fn != nil {
			input.Cmd = &cmd
			if err := cmd.Fn(&input); err != nil {
				panic(err)
			}
		} else {
			// check if it's an executable refercenced in the path
			if err := commands["type"].Fn(&input); err != nil {
				panic(err)
			}
			if input.Exe != "" {
				execute(&input)
			}
		}
	}

}
