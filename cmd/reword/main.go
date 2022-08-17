package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

func main() {
	filename := os.Args[1]

	if !strings.HasSuffix(filename, "COMMIT_EDITMSG") {
		readfile, err := os.Open(filename)
		check(err)

		lines := []string{}
		scanner := bufio.NewScanner(readfile)
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
		}
		readfile.Close()
		check(scanner.Err())

		writefile, err := os.Create(filename)
		check(err)

		for _, line := range lines {
			line := strings.Replace(line, "pick ", "reword ", 1)
			writefile.WriteString(line + "\n")
		}
		writefile.Close()
	} else {
		if shouldAppendCommitID(filename) {
			appendCommitID(filename)
		}
	}
}

func shouldAppendCommitID(filename string) bool {
	readfile, err := os.Open(filename)
	check(err)
	defer readfile.Close()

	i := 0
	nonEmptyCommitMessage := false
	scanner := bufio.NewScanner(readfile)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" && !strings.HasPrefix(line, "#") {
			nonEmptyCommitMessage = true
		}
		if strings.HasPrefix(line, "commit-id:") {
			return false
		}
		i++
	}
	check(scanner.Err())
	return nonEmptyCommitMessage
}

func appendCommitID(filename string) {
	appendfile, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0666)
	check(err)
	defer appendfile.Close()

	commitID := uuid.New()
	appendfile.WriteString("\n")
	appendfile.WriteString(fmt.Sprintf("commit-id:%s\n", commitID.String()[:8]))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
