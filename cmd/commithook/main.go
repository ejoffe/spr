package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
)

func main() {
	filename := os.Args[1]
	readfile, err := os.Open(filename)
	check(err)
	defer readfile.Close()

	scanner := bufio.NewScanner(readfile)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "commit-id:") {
			return
		}
	}
	check(scanner.Err())

	readfile.Close()
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
