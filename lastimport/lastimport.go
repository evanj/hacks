package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func checkGitGrep(importPath string) error {
	proc := exec.Command("git", "grep", `"`+importPath+`"`)
	out, err := proc.CombinedOutput()
	if err != nil {
		var exitStatus *exec.ExitError
		if errors.As(err, &exitStatus) {
			if exitStatus.ExitCode() == 1 && len(exitStatus.Stderr) == 0 {
				// no results found! this is "success"
				return nil
			}
		}
		return err
	}

	return fmt.Errorf("found imports:\n" + string(out))
}

func mostRecentImport(importPath string) (string, error) {
	proc := exec.Command("git", "log", "-p")
	proc.Stderr = os.Stderr
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer stdout.Close()
	err = proc.Start()
	if err != nil {
		return "", err
	}

	importSubstring := []byte(`"` + importPath + `"`)
	commitPrefix := []byte("commit ")

	scanner := bufio.NewScanner(stdout)
	const scanBufferSize = 32 * 1 << 20
	scanner.Buffer(make([]byte, scanBufferSize), scanBufferSize)
	var lastCommit string
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}

		if bytes.HasPrefix(lineBytes, commitPrefix) {
			parts := bytes.Split(lineBytes, []byte(" "))
			lastCommit = string(parts[1])
			continue
		}

		// search for removing an import
		if lineBytes[0] != '-' {
			continue
		}
		if bytes.Contains(lineBytes, importSubstring) {
			// found it! kill the git process
			err = proc.Process.Kill()
			if err != nil {
				return "", err
			}
			err = stdout.Close()
			if err != nil {
				return "", err
			}
			// ignore the return value: it will be an error (killed or broken pipe)
			_ = proc.Wait()

			return lastCommit, nil
		}
	}
	if scanner.Err() != nil {
		return "", scanner.Err()
	}

	err = stdout.Close()
	if err != nil {
		return "", err
	}
	err = proc.Wait()
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("import not found")
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "USAGE: lastimport (import path)\n")
		os.Exit(1)
	}
	importPath := os.Args[1]

	log.Printf("checking that there are currently no imports of %s ...", importPath)
	err := checkGitGrep(importPath)
	if err != nil {
		panic(err)
	}

	log.Printf("searching for the commit that removed the import ...")
	commit, err := mostRecentImport(importPath)
	if err != nil {
		panic(err)
	}
	log.Printf("last import removed in commit %s", commit)
}
