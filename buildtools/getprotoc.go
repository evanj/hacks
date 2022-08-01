package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "21.4"
const protocURLTemplate = "https://github.com/protocolbuffers/protobuf/releases/download/v%s/protoc-%s-%s-x86_64.zip"
const protocZipPath = "bin/protoc"
const includeZipPath = "include/"

var goosToProtocOS = map[string]string{
	"darwin": "osx",
	"linux":  "linux",
}

// computed with sha256
var protocHashes = map[string]string{
	"darwin": "27ac01aee3e8b95ebec017b7b3aee55d4eb095cbd2a5148d2a20350af006072e",
	"linux":  "d51e8f030162f08823a4738ab0ac00bee537e30b583a562e6962dbb040d86736",
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

func computeProtocHashes() error {
	for goos := range goosToProtocOS {
		log.Printf("computing hash for OS=%s ...", goos)
		protocZipBytes, err := downloadProtocForGOOS(goos)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(protocZipBytes)
		hashHex := hex.EncodeToString(hash[:])
		log.Printf("### OS=%s hash=%s", goos, hashHex)
	}
	return nil
}

func downloadProtocForGOOS(goos string) ([]byte, error) {
	protocURL := fmt.Sprintf(protocURLTemplate, version, version, goosToProtocOS[goos])
	log.Printf("downloading protoc from %s ...", protocURL)
	resp, err := http.Get(protocURL)
	if err != nil {
		return nil, err
	}
	protocZipBytes, err := ioutil.ReadAll(resp.Body)
	err2 := resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if err2 != nil {
		return nil, err2
	}
	return protocZipBytes, nil
}

func main() {
	outputDir := flag.String("outputDir", "", "Path to write bin/protoc and include/*")
	computeHashes := flag.Bool("computeHashes", false, "Downloads and print hashes for all OSes")
	flag.Parse()

	if *computeHashes {
		err := computeProtocHashes()
		if err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	expectedHash, err := hex.DecodeString(protocHashes[runtime.GOOS])
	if err != nil {
		panic(err)
	}
	protocZipBytes, err := downloadProtocForGOOS(runtime.GOOS)
	if err != nil {
		panic(err)
	}
	hash := sha256.Sum256(protocZipBytes)
	if !bytes.Equal(expectedHash, hash[:]) {
		fmt.Fprintf(os.Stderr, "Error: expected protoc hash=%s; downloaded hash=%s\n",
			protocHashes[runtime.GOOS], hex.EncodeToString(hash[:]))
		os.Exit(1)
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
