package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanj/hacks/dltools"
)

const version = "28.2"

const protocURLTemplate = "https://github.com/protocolbuffers/protobuf/releases/download/v{{.Version}}/protoc-{{.Version}}-{{.OS}}-{{.Arch}}.zip"
const protocZipPath = "bin/protoc"
const includeZipPath = "include/"

var goosToProtocOS = map[string]string{
	dltools.DARWIN: "osx",
	dltools.LINUX:  "linux",
}
var goarchToProtocArch = map[string]string{
	dltools.AMD64: "x86_64",
	dltools.ARM64: "aarch_64",
}

// computed with sha256
var protocHashes = map[dltools.Platform]string{
	{GOOS: "darwin", GOARCH: "amd64"}: "232f07d12bf4806207a79ec2c7378301c52e6f2f7efdd21c0dd416f0bda103ec",
	{GOOS: "darwin", GOARCH: "arm64"}: "7bb048f52841789d9ec61983be0ce4c9e4fb3bd9a143462820ba9a3be0a03797",
	{GOOS: "linux", GOARCH: "amd64"}:  "2febfd42b59ce93a28eb789019a470a3dd0449619bc04f84dad1333da261dec1",
	{GOOS: "linux", GOARCH: "arm64"}:  "91d8253cdc0f0f0fc51c2b69c80677996632f525ad84504bfa5b4ee38ad3e49c",
}

func shouldExtract(name string) bool {
	return !strings.HasSuffix(name, "/") &&
		(name == protocZipPath || strings.HasPrefix(name, includeZipPath))
}

func extractFromZip(outputDir string, f *zip.File) error {
	outputPath := filepath.Join(outputDir, f.Name)
	log.Printf("writing %s ...", outputPath)
	basePath := filepath.Dir(outputPath)
	err := os.MkdirAll(basePath, 0700)
	if err != nil {
		return err
	}

	// check that file is writable: remove if not to avoid permission denied
	stat, err := os.Stat(outputPath)
	if err == nil && (stat.Mode().Perm()&0200) == 0 {
		// file exists but is not writable: attempt to remove
		// explicitly ignore error: OpenFile will error if this fails
		os.Remove(outputPath)
	}

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
	if err != nil {
		return err
	}
	defer outputFile.Close()

	fileReader, err := f.Open()
	if err != nil {
		return err
	}
	defer fileReader.Close()

	_, err = io.Copy(outputFile, fileReader)
	return err
}

func main() {
	outputDir := flag.String("outputDir", "", "Path to write bin/protoc and include/*")
	computeHashes := flag.Bool("computeHashes", false, "Downloads and print hashes for all OSes")
	flag.Parse()

	fetcher, err := dltools.NewPackageFetcher(protocURLTemplate, protocHashes, version)
	if err != nil {
		panic(err)
	}
	err = fetcher.SetOSMap(goosToProtocOS)
	if err != nil {
		panic(err)
	}
	err = fetcher.SetArchMap(goarchToProtocArch)
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

	protocZipBytes, err := fetcher.DownloadForCurrentPlatform()
	if err != nil {
		panic(err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(protocZipBytes), int64(len(protocZipBytes)))
	if err != nil {
		panic(err)
	}
	for _, f := range zipReader.File {
		if shouldExtract(f.Name) {
			err = extractFromZip(*outputDir, f)
			if err != nil {
				panic(err)
			}
		}
	}
}
