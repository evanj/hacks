package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeDirContents(dirPath string, contents map[string]string) error {
	for path, fileContents := range contents {
		fullFilePath := filepath.Join(dirPath, path)
		dirPath := filepath.Dir(fullFilePath)
		err := os.Mkdir(dirPath, 0700)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}

		err = os.WriteFile(fullFilePath, []byte(fileContents), 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestUnimported(t *testing.T) {
	testDirContents := map[string]string{
		"go.mod":           "module example.com/test\n\n go 1.19\n",
		"unused/unused.go": "package unused\n",
		"used/used.go": `package used

func Used() {}
`,
		"main.go": `package main

import "example.com/test/used"

func main() {
	used.Used()
}
`,
	}
	testDir := t.TempDir()
	err := writeDirContents(testDir, testDirContents)
	if err != nil {
		panic(err)
	}

	// ensure the code is well formed
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = testDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}

	results, err := findUnimportedPackages(testDir, false)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"example.com/test/unused"}
	if !slices.Equal(expected, results.unimportedPackages) {
		t.Errorf("expected=%#v unimportedPackages=%#v", expected, results.unimportedPackages)
	}
	if len(results.mainPackages) != 1 {
		t.Errorf("expected 1 main package; mainPackages=%#v", results.mainPackages)
	}
	if len(results.testOnlyPackages) != 0 {
		t.Errorf("expected 0 test-only packages; testOnlyPackages=%#v", results.testOnlyPackages)
	}
}

func TestImportError(t *testing.T) {
	testDirContents := map[string]string{
		"go.mod": "module example.com/test\n\n go 1.19\n",
		"badimport/badimport.go": `package badimport

import "example.com/test"

func BadImport() {}
`,
		"main.go": `package main

func main() {}
`,
	}
	testDir := t.TempDir()
	err := writeDirContents(testDir, testDirContents)
	if err != nil {
		panic(err)
	}

	results, err := findUnimportedPackages(testDir, false)
	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("expected error due to bad import; results=%#v err=%#v", results, err)
	}
	if results != nil {
		t.Errorf("expected nil results=%#v", results)
	}

	// re-run with ignoreImportErrors
	results, err = findUnimportedPackages(testDir, true)
	if err != nil {
		t.Fatal(err)
	}
	// unimported should not be empty: it must still determine that badimport exists
	expected := []string{"example.com/test/badimport"}
	if !slices.Equal(expected, results.unimportedPackages) {
		t.Errorf("expected=%#v unimported=%#v", expected, results.unimportedPackages)
	}
}

func TestImportTestOnly(t *testing.T) {
	testDirContents := map[string]string{
		"go.mod": "module example.com/test\n\n go 1.19\n",
		"testonly/testonly.go": `package testonly

func TestOnly() {}
`,
		"main.go": `package main

func main() {}
`,
		"main_test.go": `package main

import (
	"example.com/test/testonly"
	"testing"
)

func TestSomething(t *testing.T) {
	testonly.TestOnly()
}
`,
	}
	testDir := t.TempDir()
	err := writeDirContents(testDir, testDirContents)
	if err != nil {
		panic(err)
	}

	// ensure the code is well formed
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = testDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}

	results, err := findUnimportedPackages(testDir, false)
	if err != nil {
		t.Fatal(err)
	}
	expectedMain := []string{"example.com/test"}
	if !slices.Equal(expectedMain, results.mainPackages) {
		t.Errorf("expected=%#v mainPackages=%#v", expectedMain, results.mainPackages)
	}
	if len(results.unimportedPackages) != 0 {
		t.Errorf("expected 0 unimported packages; unimportedPackages=%#v", results.unimportedPackages)
	}
	expected := []string{"example.com/test/testonly"}
	if !slices.Equal(expected, results.testOnlyPackages) {
		t.Errorf("expected=%#v testOnlyPackages=%#v", expected, results.testOnlyPackages)
	}
}
