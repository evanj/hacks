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

const version = "35.0"

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
	{GOOS: "darwin", GOARCH: "amd64"}: "3580c2d115fccb5b0239960c8f70f8da14787b1973a46b2f39c315ad71c11e01",
	{GOOS: "darwin", GOARCH: "arm64"}: "45444963204757fd3e2fbe304bc1fdadfb488d8556ff099c4cc06575eab88976",
	{GOOS: "linux", GOARCH: "amd64"}:  "a45cda0989c17dd950db55f6fbe1e5814c50fda08e87aa422980ac1f89dddbbc",
	{GOOS: "linux", GOARCH: "arm64"}:  "36b518ac14d90351cc6598228ed2bbe5afe4e357b1af470b07e0ec1609875de2",
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
