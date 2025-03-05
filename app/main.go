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

func main() {

	for true {
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)
		args := strings.Split(input, " ")
		if len(input) >= 0 {
			if args[0] == "exit" {
				num, err := strconv.Atoi(args[1])
				if err != nil {
					panic(err)
				}
				os.Exit(num)
			} else {
				fmt.Fprintf(os.Stdout, "%v: command not found\n", input)
			}
		} else {
			fmt.Fprintf(os.Stdout, "%v: command not found\n", input)

		}
	}

}
