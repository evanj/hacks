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

const version = "25.1"

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
	{GOOS: "darwin", GOARCH: "amd64"}: "72c6d6b2bc855ff8688c3b7fb31288ccafd0ab55256ff8382d5711ecfcc11f4f",
	{GOOS: "darwin", GOARCH: "arm64"}: "320308ce18c359564948754f51748de41cf02a4e7edf0cf47a805b9d38610f16",
	{GOOS: "linux", GOARCH: "amd64"}:  "ed8fca87a11c888fed329d6a59c34c7d436165f662a2c875246ddb1ac2b6dd50",
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
