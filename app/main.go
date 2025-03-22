package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
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

	//fmt.Println("_", input.Args[1], "_")
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
	AppendRedirect bool
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
		if p.AppendRedirect {
			return fmt.Sprintf("%v>>(%v)", p.RedirectId, p.Body)
		}
		return fmt.Sprintf("%v>(%v)", p.RedirectId, p.Body)
	}
	if p.Separator {
		return "SEP"
	}
	return fmt.Sprintf("Part(%v)", p.Body)
}

func autoComplete(input *CommandArgs, token string) (bool, string, string) {
	for key := range input.Cmds {
		if strings.HasPrefix(key, token) {
			return true, key, strings.TrimPrefix(key, token)
		}
	}

	for _, path := range input.Path {
		if entries, err := os.ReadDir(path); err == nil {
			for _, entry := range entries {
				if entry.Type().IsRegular() {
					info, err := entry.Info()
					if err == nil && (info.Mode()&0111) != 0 && strings.HasPrefix(entry.Name(), token) {
						return true, entry.Name(), strings.TrimPrefix(entry.Name(), token)
					}
				}
			}
		}
	}

	return false, "", ""
}

func processParts(input *CommandArgs) []Part {
	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		panic(err)
	}
	defer term.Restore(fd, state)

	rawCmd := ""
	token := ""
	parts := []Part{}
	in_quotes := false
	inDoubleQuotes := false
	toEscape := false
	redirect := false
	redirectId := "1"
	appendRedirect := false

	for {
		buf := make([]byte, 1)
		_, err = os.Stdin.Read(buf)
		if err != nil {
			panic(err)
		}
		char := string(buf)
		if char == "\t" {
			// don't output
		} else if char == "\r" {
			//fmt.Printf("\n\r")
			char = "\n"
		} else {
			fmt.Printf(char)
			rawCmd += char
		}

		if char == "\t" {
			if len(parts) == 0 {
				hasMatch, _, rem := autoComplete(input, token)
				if hasMatch {
					fmt.Printf("%v ", rem)
					token += rem
					rawCmd += rem + " "
					char = " "
				} else {
					fmt.Printf("\a")
				}
			}
		}

		if char == ">" && !in_quotes && !inDoubleQuotes && !toEscape {
			if redirect {
				appendRedirect = true
				token = ""
				continue
			}
			redirect = true
			if len(token) > 0 {
				if token[len(token)-1] == '1' || token[len(token)-1] == '2' {
					redirectId = string(token[len(token)-1])
					token = token[:len(token)-1]
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
				parts = append(parts, Part{Body: token, Redirect: true, RedirectId: redirectId, AppendRedirect: appendRedirect})
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

		if char == "\n" {
			break
		}
		token += char
	}
	if len(token) > 0 {
		parts = append(parts, Part{Body: token, Redirect: redirect, RedirectId: redirectId, AppendRedirect: appendRedirect})
	}

	if len(parts) > 0 {
		parts[0].IsCommand = true
	}
	input.Raw = strings.TrimSpace(rawCmd)
	input.Args = strings.Split(input.Raw, " ")

	return parts
}

func buildCommandArgs(path []string, startingDir string, env map[string]string) *CommandArgs {
	input := CommandArgs{
		Cmds:        commands,
		Path:        path,
		StartingDir: startingDir,
		Env:         env,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}

	input.Parts = processParts(&input)
	fmt.Print("\r")

	//fmt.Println(input.Parts)
	newParts := []Part{}
	for _, part := range input.Parts {
		if part.Redirect {
			openFileFlag := os.O_WRONLY | os.O_CREATE
			if part.AppendRedirect {
				openFileFlag |= os.O_APPEND
			} else {
				openFileFlag |= os.O_TRUNC
			}
			file, err := os.OpenFile(part.Body, openFileFlag, 0644)
			if err != nil {
				panic(err)
			}
			if part.RedirectId == "1" {
				input.Stdout = file
			} else {
				input.Stderr = file
			}
		} else {
			newParts = append(newParts, part)
		}
	}

	input.Parts = newParts
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

		input := buildCommandArgs(path, startingDir, env)

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
