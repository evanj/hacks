package main

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFromZip(t *testing.T) {
	// create a zip with a file with permissions r--r--r-- like protoc
	zipBuf := &bytes.Buffer{}
	zipWriter := zip.NewWriter(zipBuf)
	header := &zip.FileHeader{Name: "example.txt"}
	header.SetMode(fs.FileMode(0400))
	fh, err := zipWriter.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	fh.Write([]byte("abc"))
	err = zipWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	// read the zip to get the file back
	zipReader, err := zip.NewReader(bytes.NewReader(zipBuf.Bytes()), int64(zipBuf.Len()))
	if err != nil {
		panic(err)
	}
	if len(zipReader.File) != 1 {
		t.Fatalf("expected 1 file in zip was %d", len(zipReader.File))
	}

	tempDir := t.TempDir()
	zipFile := zipReader.File[0]
	err = extractFromZip(tempDir, zipFile)
	if err != nil {
		t.Fatal(err)
	}

	// extracting a second time should work: used to get permission denied
	err = extractFromZip(tempDir, zipFile)
	if err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(tempDir, header.Name))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "abc" {
		t.Errorf("unexpected contents: %#v", string(contents))
	}
}
