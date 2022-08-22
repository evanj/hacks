package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const maxLevel = 50

type importSearcher struct {
	visited map[string]struct{}
}

func newImportSearcher() *importSearcher {
	return &importSearcher{make(map[string]struct{})}
}

// isStandardLibrary returns true if packagePath is a standard library import
func isStandardLibrary(packagePath string) bool {
	if packagePath == "" {
		return false
	}

	// check if the first part looks like a domain name: contains a .
	parts := strings.SplitN(packagePath, "/", 2)
	return !strings.Contains(parts[0], ".")
}

func (i *importSearcher) shouldVisit(packagePath string) bool {
	// do not visit standard library imports
	if isStandardLibrary(packagePath) {
		return false
	}
	if _, exists := i.visited[packagePath]; exists {
		return false
	}
	i.visited[packagePath] = struct{}{}
	return true
}

func getPackageImports(packagePath string) ([]string, error) {
	proc := exec.Command("go", "list", "-f", "{{range .Imports}}{{.}}\n{{end}}", packagePath)
	proc.Stderr = os.Stderr
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdout.Close()

	err = proc.Start()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdout)
	var out []string
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		out = append(out, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	err = stdout.Close()
	if err != nil {
		return nil, err
	}
	err = proc.Wait()
	if err != nil {
		return nil, err
	}

	return out, nil
}

const baseIndent = "  "

func makeIndent(level int) string {
	var output strings.Builder
	output.Grow(level * len(baseIndent))
	for i := 0; i < level; i++ {
		output.WriteString(baseIndent)
	}
	return output.String()
}

func (i *importSearcher) recursePrintPackage(packagePath string, level int) error {
	indent := makeIndent(level)
	fmt.Printf("%s%s\n", indent, packagePath)

	if level >= maxLevel {
		return nil
	}

	importPaths, err := getPackageImports(packagePath)
	if err != nil {
		return err
	}

	for _, importPath := range importPaths {
		if !i.shouldVisit(importPath) {
			continue
		}

		err = i.recursePrintPackage(importPath, level+1)
		if err != nil {
			return err
		}
	}
	return nil
}

const usageMessage = `importtree: Print the complete import tree starting at a Go package

Usage: importtree (package path)
`

func main() {
	if len(os.Args) != 2 {
		fmt.Fprint(os.Stderr, usageMessage)
		os.Exit(1)
	}
	packagePath := os.Args[1]

	// TODO: configure package filters
	searcher := newImportSearcher()
	err := searcher.recursePrintPackage(packagePath, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
