package main

import (
	"bufio"
	"os"
	"strings"
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
		file, err := os.Open(filename)
		check(err)
		file.Close()
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
