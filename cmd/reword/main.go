// TODO HARIOM: this is using interactive rebase to add commit-id to the commit message.
// it does this by changing pick to reword in the commit message file.
// as shown here: https://docs.github.com/en/pull-requests/committing-changes-to-your-project/creating-and-editing-commits/changing-a-commit-message
// the github api only allows us to update the commit message of the merge commit
// as such, older commits in the stack which are merged to main won't contain PR numbers
// use something like this to add PR numbers to commit summary of each commit before merging
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git/realgit"
	"github.com/google/uuid"
)

var PR_NUMBER_RE = regexp.MustCompile(`\(#\d+\)`)

func main() {
	prNumber := flag.Int("pr-number", -1, "PR number to add to commit message")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [--pr-number <digits>] <filename>\n", os.Args[0])
		os.Exit(1)
	}
	filename := args[0]

	gitcmd := realgit.NewGitCmd(config.DefaultConfig())
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
			if strings.HasPrefix(line, "pick") {
				res := strings.Split(line, " ")
				var out string
				gitcmd.Git("log --format=%B -n 1 "+res[1], &out)
				if !strings.Contains(out, "commit-id") {
					line = strings.Replace(line, "pick ", "reword ", 1)
				}
			}
			writefile.WriteString(line + "\n")
		}
		writefile.Close()
	} else {
		missingCommitID, missingNewLine := shouldAppendCommitID(filename)
		if missingCommitID {
			appendCommitID(filename, missingNewLine)
		}
		if *prNumber != -1 {
			appendPRNumber(filename, *prNumber)
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

func appendPRNumber(filename string, prNumber int) {
	content, err := os.ReadFile(filename)
	check(err)

	lines := strings.Split(string(content), "\n")
	line := PR_NUMBER_RE.ReplaceAll([]byte(lines[0]), []byte(""))
	lines[0] = fmt.Sprintf("%s (#%d)", strings.TrimSpace(string(line)), prNumber)

	os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0666)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
