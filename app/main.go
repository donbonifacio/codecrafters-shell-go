package main

import (
	"bufio"
	"fmt"
	"io"
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
	Parts       []Part
	Path        []string
	Env         map[string]string
	Exe         string
	StartingDir string
	Stdout      io.Writer
	Stderr      io.Writer
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
	if len(input.Parts) <= 1 {
		fmt.Fprintln(input.Stdout, "")
		return nil
	}
	for i, part := range input.Parts[1:] {
		fmt.Fprint(input.Stdout, part.Body)

		if part.Escaped {
			continue
		}

		if part.InQuotes || part.InDoubleQuotes || part.Separator {
			continue
		}

		var nextPart *Part
		if i+2 < len(input.Parts) {
			nextPart = &input.Parts[i+2]
		}
		if nextPart != nil && !nextPart.Escaped && !nextPart.InQuotes && !nextPart.InDoubleQuotes && !nextPart.Separator {
			fmt.Fprint(input.Stdout, " ")
		}
	}
	fmt.Fprintln(input.Stdout, "")
	return nil
}

func typeFn(input *CommandArgs) error {
	target := input.Parts[0].Body
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

func buildArgsFromParts(input *CommandArgs) []string {
	var args []string
	for _, part := range input.Parts {
		if part.InQuotes {
			args = append(args, fmt.Sprintf("%s", part.Body))
		} else if part.Separator {
			continue
		} else {
			args = append(args, part.Body)
		}
	}
	return args
}

func execute(input *CommandArgs) error {
	exe := input.Exe
	cmd := exec.Command(exe)
	if len(input.Args) > 1 {
		cmd.Args = buildArgsFromParts(input)
	}
	cmd.Stdout = input.Stdout
	cmd.Stderr = input.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func unknownCommand(input *CommandArgs) error {
	fmt.Fprintf(os.Stdout, "%v: command not found\n", input.Raw)
	return nil
}

type Part struct {
	Body           string
	IsCommand      bool
	InQuotes       bool
	InDoubleQuotes bool
	Separator      bool
	Escaped        bool
	Redirect       bool
	RedirectId     string
}

func (p Part) String() string {
	if p.IsCommand {
		return fmt.Sprintf("Cmd(%v)", p.Body)
	}
	if p.InQuotes {
		return fmt.Sprintf("'(%v)", p.Body)
	}
	if p.InDoubleQuotes {
		return fmt.Sprintf("\"(%v)", p.Body)
	}
	if p.Escaped {
		return fmt.Sprintf("\\(%v)", p.Body)
	}
	if p.Redirect {
		return fmt.Sprintf("%v>(%v)", p.RedirectId, p.Body)
	}
	if p.Separator {
		return "SEP"
	}
	return fmt.Sprintf("Part(%v)", p.Body)
}

func processParts(raw string) []Part {
	chars := []rune(raw)
	token := ""
	parts := []Part{}
	in_quotes := false
	inDoubleQuotes := false
	toEscape := false
	redirect := false
	redirectId := "1"

	for _, c := range chars {
		char := string(c)
		if char == ">" && !in_quotes && !inDoubleQuotes && !toEscape {
			redirect = true
			if len(token) > 0 {
				if token[len(token)-1] == '1' {
					token = token[:len(token)-1]
					redirectId = "1"
				}
			}
			if len(token) > 0 {
				parts = append(parts, Part{Body: token})
			}
			token = ""
			continue
		}
		if char == "\\" && !toEscape {
			toEscape = true
			continue
		}
		if toEscape {
			toEscape = false
			if in_quotes || (inDoubleQuotes) {
				if inDoubleQuotes && strings.Contains("$\"\\", char) {
					token += char
				} else {
					token += "\\" + char
				}
				continue
			}
			if len(token) > 0 {
				parts = append(parts, Part{Body: token})
				token = ""
			}
			parts = append(parts, Part{Body: char, Escaped: true})
			continue
		}

		if char == "'" && !inDoubleQuotes {
			if in_quotes == false {
				in_quotes = true
			} else {
				in_quotes = false
				parts = append(parts, Part{Body: token, InQuotes: true})
				token = ""
			}
			continue
		}
		if char == "\"" && !in_quotes {
			if inDoubleQuotes == false {
				inDoubleQuotes = true
			} else {
				inDoubleQuotes = false
				parts = append(parts, Part{Body: token, InDoubleQuotes: true})
				token = ""
			}
			continue
		}
		if char == " " {
			if in_quotes == true || inDoubleQuotes == true {
				token += string(char)
			} else if redirect && len(strings.TrimSpace(token)) > 0 {
				redirect = false
				parts = append(parts, Part{Body: token, Redirect: true, RedirectId: redirectId})
			} else {
				if strings.TrimSpace(token) == "" {
					var lastPart *Part
					if len(parts) > 0 {
						lastPart = &parts[len(parts)-1]
					}
					if lastPart == nil || !lastPart.Separator {
						parts = append(parts, Part{Separator: true, Body: " "})
					}
				} else {
					parts = append(parts, Part{Body: token})
				}
				token = ""
			}
			continue
		}
		token += char
	}
	if len(token) > 0 {
		parts = append(parts, Part{Body: token, Redirect: redirect, RedirectId: redirectId})
	}

	if len(parts) > 0 {
		parts[0].IsCommand = true
	}
	return parts
}

func buildCommandArgs(raw string, path []string, startingDir string, env map[string]string) *CommandArgs {
	input := CommandArgs{
		Cmds:        commands,
		Raw:         raw,
		Path:        path,
		StartingDir: startingDir,
		Env:         env,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}

	rawCmd := raw
	input.Args = strings.Split(rawCmd, " ")
	input.Parts = processParts(rawCmd)

	//fmt.Println(input.Parts)
	newParts := []Part{}
	for _, part := range input.Parts {
		if part.Redirect {
			file, err := os.OpenFile(part.Body, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				panic(err)
			}
			input.Stdout = file
		} else {
			newParts = append(newParts, part)
		}
	}

	input.Parts = newParts
	//fmt.Println(input.Parts)
	return &input
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

		input := buildCommandArgs(raw, path, startingDir, env)

		cmd := commands[input.Parts[0].Body]
		if cmd.Fn != nil {
			input.Cmd = &cmd
			if err := cmd.Fn(input); err != nil {
				panic(err)
			}
		} else {
			// check if it's an executable referenced in the path
			if err := commands["type"].Fn(input); err != nil {
				panic(err)
			}
			if input.Exe != "" {
				execute(input)
			}
		}
	}
}
