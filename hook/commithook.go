package hook

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
)

func CommitHook(filename string) {
	readfile, err := os.Open(filename)
	check(err)
	defer readfile.Close()

	// scan for commit-id - if found do nothing and exit
	scanner := bufio.NewScanner(readfile)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "commit-id:") {
			return
		}
	}
	check(scanner.Err())
	readfile.Close()

	// commit-id not found - append a new commit-id to the end of the file
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
