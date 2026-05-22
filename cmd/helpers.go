package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var stdinReader *bufio.Reader

func resetStdinReader() {
	stdinReader = nil
}

func getStdinReader() *bufio.Reader {
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	return stdinReader
}

func readLine() (string, error) {
	text, err := getStdinReader().ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func readPassword() (string, error) {
	return readLine()
}

func confirm(prompt string) bool {
	fmt.Print(prompt + " [y/N]: ")
	input, err := readLine()
	if err != nil {
		return false
	}
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}
