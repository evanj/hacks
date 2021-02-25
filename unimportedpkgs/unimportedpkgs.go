package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
)

type packageInfo struct {
	importPath string
	name       string
	imports    []string
}

func getPackageInfo() ([]packageInfo, error) {
	proc := exec.Command("go", "list", "-f", "{{.ImportPath}} {{.Name}}{{range .Imports}} {{.}}{{end}}", "./...")
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
	var out []packageInfo
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		parts := bytes.Split(scanner.Bytes(), []byte(" "))
		importPath := string(parts[0])
		name := string(parts[1])

		imports := make([]string, len(parts)-2)
		for i, importBytes := range parts[2:] {
			imports[i] = string(importBytes)
		}
		out = append(out, packageInfo{importPath, name, imports})
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

func sortedStrings(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func main() {
	log.Printf("getting imports for all packages in current directory ...")
	pkgs, err := getPackageInfo()
	if err != nil {
		panic(err)
	}
	log.Printf("found %d Go packages", len(pkgs))

	// 1. add all packages we found to a map
	// 2. remove the packages that are imported
	unimportedPkgs := make(map[string]struct{}, len(pkgs))
	mainPkgs := make(map[string]struct{})
	for _, pkg := range pkgs {
		if pkg.name == "main" {
			mainPkgs[pkg.importPath] = struct{}{}
		} else {
			unimportedPkgs[pkg.importPath] = struct{}{}
		}
	}
	if len(unimportedPkgs)+len(mainPkgs) != len(pkgs) {
		panic(fmt.Sprintf("BUG: len(unimportedPkgs)=%d; len(mainPkgs)=%d; len(pkgs)=%d",
			len(unimportedPkgs), len(mainPkgs), len(pkgs)))
	}

	for _, pkg := range pkgs {
		for _, importedPath := range pkg.imports {
			delete(unimportedPkgs, importedPath)
		}
	}

	log.Printf("## %d MAIN PACKAGES:", len(mainPkgs))
	for _, pkg := range sortedStrings(mainPkgs) {
		log.Println("  ", pkg)
	}

	log.Printf("## %d UNIMPORTED PACKAGES:", len(unimportedPkgs))
	for _, pkg := range sortedStrings(unimportedPkgs) {
		log.Println("  ", pkg)
	}
}
