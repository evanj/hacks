package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/evanj/hacks/dltools"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
)

const nodeVersion = "18.16.0"
const nodeURLTemplate = "https://nodejs.org/dist/v{{.Version}}/node-v{{.Version}}-{{.OS}}-{{.Arch}}.tar.xz"

var goarchToNode = map[string]string{
	"amd64": "x64",
	"arm64": "arm64",
}

// computed with sha256
var nodeHashes = map[dltools.Platform]string{
	{GOOS: "darwin", GOARCH: "amd64"}: "2c8aa0333111c2411564bfb85be44186aeb581392f73c4be5912cbb125d99043",
	{GOOS: "darwin", GOARCH: "arm64"}: "b23cdf4fa0e9f77273720ab18eabdd7691edbb69e08ec3b65afd69bef23fe209",
	{GOOS: "linux", GOARCH: "amd64"}:  "44d93d9b4627fe5ae343012d855491d62c7381b236c347f7666a7ad070f26548",
}

func installTypescript(fetcher *dltools.PackageFetcher, nodeDir string, logf dltools.LogFunc) error {
	log.Printf("installing node and typescript in dir=%s ...", nodeDir)

	nodePackageBytes, err := fetcher.DownloadForCurrentPlatform()
	if err != nil {
		return err
	}

	log.Printf("extracting %d bytes to %s ...", len(nodePackageBytes), nodeDir)

	r, err := xz.NewReader(bytes.NewReader(nodePackageBytes))
	if err != nil {
		return err
	}
	err = dltools.ExtractTar(r, nodeDir, logf)
	if err != nil {
		return err
	}

	// install typescript using npm into the npm path
	log.Printf("installing typescript with npm ...")
	npmPath := getNodeExePath(nodeDir, "npm")
	cmd := exec.Command(npmPath, "install", "typescript", "--global")
	cmd.Env = getEnvToNode(nodeDir, logf)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logf("running %s ...", strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func getNodeExePath(nodeDir string, exe string) string {
	nodeBinDir := filepath.Join(nodeDir, "bin")
	return filepath.Join(nodeBinDir, exe)
}

func getEnvToNode(nodeDir string, logf dltools.LogFunc) []string {
	nodeBinDir := filepath.Join(nodeDir, "bin")

	env := os.Environ()
	for i, envVar := range env {
		const pathEnvVarPrefix = "PATH="
		if strings.HasPrefix(envVar, pathEnvVarPrefix) {
			// our node bin dir must go first: npm uses "/usr/bin/env node"
			pathValue := envVar[len(pathEnvVarPrefix):]
			env[i] = pathEnvVarPrefix + nodeBinDir + string(filepath.ListSeparator) + pathValue
			logf("setting PATH: %s", env[i])
		}
	}

	return env
}

func execTypescript(nodeDir string, logf dltools.LogFunc, extraArgs []string) error {
	env := getEnvToNode(nodeDir, logf)
	args := []string{getNodeExePath(nodeDir, "tsc")}
	args = append(args, extraArgs...)
	logf("calling Exec: %s", strings.Join(args, " "))
	return unix.Exec(args[0], args, env)
}

func main() {
	nodeDir := flag.String("nodeDir", "", "Path to write node directory containing node and typescript")
	computeHashes := flag.Bool("computeHashes", false, "Downloads and print hashes for all OSes")
	verbose := flag.Bool("verbose", false, "Enables verbose logging")
	flag.Parse()

	fetcher, err := dltools.NewPackageFetcher(nodeURLTemplate, nodeHashes, nodeVersion)
	if err != nil {
		panic(err)
	}
	err = fetcher.SetArchMap(goarchToNode)
	if err != nil {
		panic(err)
	}

	if *computeHashes {
		hashes, err := fetcher.ComputeHashes()
		if err != nil {
			panic(err)
		}
		os.Stdout.WriteString(dltools.FormatHashes(hashes))
		os.Exit(0)
	}

	if *nodeDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: runtypescript --nodeDir=(nodedir)\n\n")
		fmt.Fprintf(os.Stderr, "  nodeDir: Path to write node directory containing node and typescript\n")
		os.Exit(1)
	}

	logf := dltools.NilLogFunc
	if *verbose {
		logf = log.Printf
	}

	statResult, err := os.Stat(*nodeDir)
	if os.IsNotExist(err) {
		err = installTypescript(fetcher, *nodeDir, logf)
		if err != nil {
			panic(err)
		}
	} else if !statResult.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: nodeDir=%s: must not exist but is a file\n",
			*nodeDir)
		os.Exit(1)
	}

	// nodeDir exists and we will assume it contains node
	err = execTypescript(*nodeDir, logf, flag.Args())
	if err != nil {
		panic(err)
	}
	// this should not return
	panic("BUG: should not get here")
}
