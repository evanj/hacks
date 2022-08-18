package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

	unimported, err := findUnimportedPackages(testDir)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"example.com/test/unused"}
	if !reflect.DeepEqual(expected, unimported) {
		t.Errorf("expected=%#v unimported=%#v", expected, unimported)
	}
}
