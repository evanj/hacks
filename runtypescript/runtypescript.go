package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/evanj/hacks/dltools"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
)

const nodeVersion = "18.12.1"
const nodeURLTemplate = "https://nodejs.org/dist/v%s/node-v%s-%s-%s.tar.xz"

type platform struct {
	goos   string
	goarch string
}

func (p platform) String() string {
	return fmt.Sprintf("GOOS=%s GOARCH=%s", p.goos, p.goarch)
}

func getPlatform() platform {
	return platform{runtime.GOOS, runtime.GOARCH}
}

var goarchToNodePlatform = map[string]string{
	"amd64": "x64",
	"arm64": "arm64",
}

// computed with sha256
var nodeHashes = map[platform]string{
	{"linux", "amd64"}:  "4481a34bf32ddb9a9ff9540338539401320e8c3628af39929b4211ea3552a19e",
	{"darwin", "amd64"}: "6c88d462550a024661e74e9377371d7e023321a652eafb3d14d58a866e6ac002",
	{"darwin", "arm64"}: "17f2e25d207d36d6b0964845062160d9ed16207c08d09af33b9a2fd046c5896f",
}

func installTypescript(nodeDir string, logf dltools.LogFunc) error {
	log.Printf("installing node and typescript in dir=%s ...", nodeDir)

	hostPlatform := getPlatform()

	expectedHash := nodeHashes[hostPlatform]
	if expectedHash == "" {
		return fmt.Errorf("missing expected hash %s", hostPlatform.String())
	}

	nodePlatform := goarchToNodePlatform[hostPlatform.goarch]
	if nodePlatform == "" {
		return fmt.Errorf("missing node platform for GOARCH=%s", hostPlatform.goarch)
	}

	nodeURL := fmt.Sprintf(nodeURLTemplate, nodeVersion, nodeVersion, runtime.GOOS, nodePlatform)
	log.Printf("downloading url=%s ...", nodeURL)
	nodePackageBytes, err := dltools.Download(nodeURL, expectedHash)
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

	// nodeModulesDir := filepath.Join(nodeDir, "node_modules")
	// nodePathEnvVar := "NODE_PATH=" + nodeModulesDir
	// logf("setting %s", nodePathEnvVar)
	// env = append(env, nodePathEnvVar)

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
	if *nodeDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: runtypescript --nodeDir=(nodedir)\n\n")
		fmt.Fprintf(os.Stderr, "  nodeDir: Path to write node directory containing node and typescript\n")
		os.Exit(1)
	}

	if *computeHashes {
		panic("TODO: implement computeHashes")
	}

	logf := dltools.NilLogFunc
	if *verbose {
		logf = log.Printf
	}

	statResult, err := os.Stat(*nodeDir)
	if os.IsNotExist(err) {
		err = installTypescript(*nodeDir, logf)
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
