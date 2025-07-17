package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
)

type packageInfo struct {
	importPath  string
	name        string
	imports     []string
	testImports []string
}

func getPackageInfo(dirPath string, ignoreImportErrors bool) ([]packageInfo, error) {
	const testImportsSeparator = "||TESTIMPORTS||"
	command := []string{"go", "list"}
	if ignoreImportErrors {
		command = append(command, "-e")
	}
	command = append(command, "-f",
		"{{.ImportPath}} {{.Name}}{{range .Imports}} {{.}}{{end}} "+testImportsSeparator+"{{range .TestImports}} {{.}}{{end}}",
		"./...")
	proc := exec.Command(command[0], command[1:]...)
	proc.Dir = dirPath
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

		imports := make([]string, 0, len(parts)-2)
		var testImports []string
		for i, importBytes := range parts[2:] {
			if bytes.Equal(importBytes, []byte(testImportsSeparator)) {
				remainingParts := parts[2+i+1:]
				testImports = make([]string, 0, len(remainingParts))
				for _, importBytes := range remainingParts {
					testImports = append(testImports, string(importBytes))
				}
				break
			}
			imports = append(imports, string(importBytes))
		}
		out = append(out, packageInfo{importPath, name, imports, testImports})
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

type packageTypes struct {
	// mainPackages are named "main" and are executable programs.
	mainPackages []string
	// unimportedPackages are not imported by anything.
	unimportedPackages []string
	// testOnlyPackages are only imported by test files.
	testOnlyPackages []string
}

type importState int

const (
	importStateUnimported importState = iota
	importStateTestOnly
)

func findUnimportedPackages(dirPath string, ignoreImportErrors bool) (*packageTypes, error) {
	log.Printf("getting imports for all packages in directory=%s ...", dirPath)
	pkgs, err := getPackageInfo(dirPath, ignoreImportErrors)
	if err != nil {
		var exitErr *exec.ExitError
		if ignoreImportErrors && errors.As(err, &exitErr) {
			log.Printf("WARNING: ignoring import errors due to -ignoreImportErrors: %s", err.Error())
		} else {
			return nil, err
		}
	}
	log.Printf("found %d Go packages", len(pkgs))

	// add all packages we found to a map with the current import state
	pkgImportState := make(map[string]importState, len(pkgs))
	var mainPkgs []string
	for _, pkg := range pkgs {
		if pkg.name == "main" {
			mainPkgs = append(mainPkgs, pkg.importPath)
		} else {
			pkgImportState[pkg.importPath] = importStateUnimported
		}
	}
	// assertion to check for bugs
	if len(pkgImportState)+len(mainPkgs) != len(pkgs) {
		panic(fmt.Sprintf("BUG: len(pkgImportState)=%d; len(mainPkgs)=%d; len(pkgs)=%d",
			len(pkgImportState), len(mainPkgs), len(pkgs)))
	}

	// check every imported package: mark a package as test-only, or remove it entirely if imported
	for _, pkg := range pkgs {
		for _, importedPath := range pkg.imports {
			delete(pkgImportState, importedPath)
		}
		for _, importedPath := range pkg.testImports {
			if _, exists := pkgImportState[importedPath]; exists {
				pkgImportState[importedPath] = importStateTestOnly
			}
		}
	}

	// collect the results
	var unimportedPkgs []string
	var testOnlyPkgs []string
	for pkg, state := range pkgImportState {
		switch state {
		case importStateUnimported:
			unimportedPkgs = append(unimportedPkgs, pkg)
		case importStateTestOnly:
			testOnlyPkgs = append(testOnlyPkgs, pkg)
		default:
			panic(fmt.Sprintf("BUG: unknown import state: %d", state))
		}
	}

	slices.Sort(mainPkgs)
	slices.Sort(unimportedPkgs)
	slices.Sort(testOnlyPkgs)
	return &packageTypes{
		mainPackages:       mainPkgs,
		unimportedPackages: unimportedPkgs,
		testOnlyPackages:   testOnlyPkgs,
	}, nil
}

func main() {
	ignoreImportErrors := flag.Bool("ignoreImportErrors", false,
		"if true: warns if go list returns an error but continues anyway")
	flag.Parse()

	results, err := findUnimportedPackages(".", *ignoreImportErrors)
	if err != nil {
		panic(err)
	}

	printSeparator := func() {
		log.Println("")
		log.Println("=================")
		log.Println("")
	}

	log.Printf("## %d MAIN PACKAGES:", len(results.mainPackages))
	for _, pkg := range results.mainPackages {
		log.Println("  ", pkg)
	}
	printSeparator()

	log.Printf("## %d TEST-ONLY PACKAGES:", len(results.testOnlyPackages))
	for _, pkg := range results.testOnlyPackages {
		log.Println("  ", pkg)
	}
	printSeparator()

	log.Printf("## %d UNIMPORTED PACKAGES:", len(results.unimportedPackages))
	for _, pkg := range results.unimportedPackages {
		log.Println("  ", pkg)
	}
}
