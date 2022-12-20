// Package dltools helps to download files and extract them.
package dltools

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Download saves URL as bytes in memory, verifying a SHA256 hash.
func Download(url string, expectedSHA256Hash string) ([]byte, error) {
	expectedHashBytes, err := hex.DecodeString(expectedSHA256Hash)
	if err != nil {
		return nil, fmt.Errorf("dltools: could not decode expected hash: %w", err)
	}
	if len(expectedHashBytes) != sha256.Size {
		return nil, fmt.Errorf("dltools: expected hash len=%d; expected %d",
			len(expectedHashBytes), sha256.Size)
	}

	bodyBytes, err := DownloadUnverified(url)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(bodyBytes)
	if !bytes.Equal(expectedHashBytes, hash[:]) {
		return nil, fmt.Errorf("expected hash=%s; downloaded hash=%s",
			expectedSHA256Hash, hex.EncodeToString(hash[:]))
	}
	return bodyBytes, nil
}

// DownloadUnverified saves URL as bytes in memory.
func DownloadUnverified(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	err2 := resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if err2 != nil {
		return nil, err2
	}
	return bodyBytes, nil
}

// LogFunc defines a function for verbose logging.
type LogFunc func(message string, args ...interface{})

// NilLogFunc does not log anything.
func NilLogFunc(message string, args ...interface{}) {}

// ExtractTar extracts a tar file from r into destinationDir.
func ExtractTar(r io.Reader, destinationDir string, logf LogFunc) error {
	tarReader := tar.NewReader(r)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		tarFileParts := strings.Split(header.Name, "/")
		tarFileParts[0] = destinationDir
		outputFilePath := filepath.Join(tarFileParts...)
		logf("tar file=%s mode=%s output path=%s...",
			header.Name, header.FileInfo().Mode().String(), outputFilePath)

		// if this is a directory entry: create it and continue
		if header.Typeflag == tar.TypeDir {
			err = os.Mkdir(outputFilePath, fs.FileMode(header.Mode))
			if err != nil {
				return err
			}
			continue
		} else if header.Typeflag == tar.TypeSymlink {
			// only allow relative symlinks
			// TODO: verify that links don't escape the unpacked root
			if path.IsAbs(header.Linkname) {
				return fmt.Errorf("tar path=%s absolute Linkname=%s", header.Name, header.Linkname)
			}
			err = os.Symlink(header.Linkname, outputFilePath)
			if err != nil {
				return err
			}
			continue
		}

		// must be a plain file
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("tar file with path=%s Typeflag=%d Reg=%d Dir=%d",
				header.Name, header.Typeflag, tar.TypeReg, tar.TypeDir)
		}

		f, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fs.FileMode(header.Mode))
		if err != nil {
			return err
		}
		_, err = io.Copy(f, tarReader)
		err2 := f.Close()
		if err != nil {
			return err
		}
		if err2 != nil {
			return err2
		}
	}
	return nil
}
