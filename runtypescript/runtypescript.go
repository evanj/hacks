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

const nodeVersion = "18.14.1"
const nodeURLTemplate = "https://nodejs.org/dist/v{{.Version}}/node-v{{.Version}}-{{.OS}}-{{.Arch}}.tar.xz"

var goarchToNode = map[string]string{
	"amd64": "x64",
	"arm64": "arm64",
}

// computed with sha256
var nodeHashes = map[dltools.Platform]string{
	{GOOS: "darwin", GOARCH: "amd64"}: "adc24c3d08e88ee7e1191d77984dd911fceb3b50d5d5042541e323ce03c820cf",
	{GOOS: "darwin", GOARCH: "arm64"}: "caeaef362c2c41ce918339f002b35c8d55dba841fc3fa26695411e8aa92b7ad9",
	{GOOS: "linux", GOARCH: "amd64"}:  "ea3bd72ad5ccaa1d9bcb40da69a60f249d29672d41740fe8f3e976d0e9078f65",
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
		if strings.HasPrefix(envVar, "PATH=") {
			env[i] = envVar + string(filepath.ListSeparator) + nodeBinDir
			logf("setting PATH: %s", env[i])
		}
	}

	return env
}

func execTypescript(nodeDir string, logf dltools.LogFunc, extraArgs []string) error {
	logf("calling Exec ...")
	env := getEnvToNode(nodeDir, logf)
	args := []string{getNodeExePath(nodeDir, "tsc")}
	args = append(args, extraArgs...)
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
