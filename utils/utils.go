package utils

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const Marker = "AIPHELPER_MARKER"

func SnakeCase(str string) string {
	str = strings.ToLower(str)
	var match1 = regexp.MustCompile(`[^a-z0-9]`)
	var match2 = regexp.MustCompile(`(_)*`)
	str = match1.ReplaceAllString(str, "_")
	str = match2.ReplaceAllString(str, "${1}")
	str = strings.Trim(str, "_")
	return str
}

func ReplaceInString(fileContents string, newSection string) (string, error) {
	lines := strings.Split(fileContents, "\n")
	newLines := strings.Split(newSection, "\n")

	beginLine, endLine := -1, -1

	for i, line := range lines {
		if beginLine == -1 && line == fmt.Sprintf("### %s_START ###", Marker) {
			beginLine = i
		}
		if line == fmt.Sprintf("### %s_END ###", Marker) {
			endLine = i
		}
	}

	var newFileContents []string
	if beginLine == -1 || endLine == -1 {
		// Append to file
		// fmt.Println("block not found. Appending new block")
		newFileContents = append(lines, newLines...)
	} else {
		// Replace block in file
		contentBefore := lines[0:beginLine]
		contentAfter := []string{}
		if len(lines) > endLine+1 {
			contentAfter = lines[endLine+1 : len(lines)-1]
		}

		newFileContents = append(contentBefore, newLines...)
		newFileContents = append(newFileContents, contentAfter...)
	}

	return strings.Join(newFileContents, "\n"), nil
}

func CreateOrReplaceInFile(path string, replaceWith string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(path), 0755)
		os.Create(path)
	}

	fileContents, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	lines := strings.Split(string(fileContents), "\n")
	newLines := strings.Split(replaceWith, "\n")

	beginLine, endLine := -1, -1

	for i, line := range lines {
		if beginLine == -1 && line == fmt.Sprintf("### %s_START ###", Marker) {
			beginLine = i
		}
		if line == fmt.Sprintf("### %s_END ###", Marker) {
			endLine = i
		}
	}

	var newFileContents []string
	if beginLine == -1 || endLine == -1 {
		// Append to file
		// fmt.Println("block not found. Appending new block")
		newFileContents = append(lines, newLines...)
	} else {
		// Replace block in file
		contentBefore := lines[0:beginLine]
		contentAfter := []string{}
		if len(lines) > endLine+1 {
			contentAfter = lines[endLine+1 : len(lines)-1]
		}

		newFileContents = append(contentBefore, newLines...)
		newFileContents = append(newFileContents, contentAfter...)
	}

	output := strings.Join(newFileContents, "\n")

	return ioutil.WriteFile(path, []byte(output), 0755)
}

func SplitArgumentParser(value string) []string {
	var delimiter = regexp.MustCompile("[, ] *")
	return delimiter.Split(value, -1)
}
