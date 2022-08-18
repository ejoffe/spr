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
		missingCommitID, missingNewLine := shouldAppendCommitID(filename)
		if missingCommitID {
			appendCommitID(filename, missingNewLine)
		}
	}
}

func shouldAppendCommitID(filename string) (missingCommitID bool, missingNewLine bool) {
	readfile, err := os.Open(filename)
	check(err)
	defer readfile.Close()

	missingCommitID = false
	missingNewLine = false

	lineCount := 0
	nonEmptyCommitMessage := false
	scanner := bufio.NewScanner(readfile)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" && !strings.HasPrefix(line, "#") {
			nonEmptyCommitMessage = true
		}
		if !strings.HasPrefix(line, "#") {
			lineCount += 1
		}
		if strings.HasPrefix(line, "commit-id:") {
			missingCommitID = false
			return
		}
	}

	if lineCount == 1 {
		missingNewLine = true
	} else {
		missingNewLine = false
	}

	check(scanner.Err())
	if nonEmptyCommitMessage {
		missingCommitID = true
	}
	return
}

func appendCommitID(filename string, missingNewLine bool) {
	appendfile, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0666)
	check(err)
	defer appendfile.Close()

	commitID := uuid.New()
	if missingNewLine {
		appendfile.WriteString("\n")
	}
	appendfile.WriteString("\n")
	appendfile.WriteString(fmt.Sprintf("commit-id:%s\n", commitID.String()[:8]))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
