package hook

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
)

// CommitHook runs the commit hook to add commit-id to file named filename
func CommitHook(filename string) {
	if shouldAppendCommitID(filename) {
		appendCommitID(filename)
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
		log.Fatal(err)
	}
}
