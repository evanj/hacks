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

const version = "22.0"

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
	{GOOS: "darwin", GOARCH: "amd64"}: "1e0ad38fcf20a4b1cdeffe40f9188c4d1c30a9dd515cf92c8b57f629227f0eb3",
	{GOOS: "darwin", GOARCH: "arm64"}: "834f35b26082ff2dc372df17cae4a4b7cded944756f1c99bac8c624214b542cc",
	{GOOS: "linux", GOARCH: "amd64"}:  "9ceff6c3945d521d1d0f42f9f57f6ef7cf3f581a9d303a027ba19b192045d1a2",
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
