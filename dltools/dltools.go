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
	"runtime"
	"sort"
	"strings"
	"text/template"
)

// AMD64 is the amd64 GOARCH value
const AMD64 = "amd64"

// ARM64 is the arm64 GOARCH value
const ARM64 = "arm64"

// DARWIN is the darwin GOOS value
const DARWIN = "darwin"

// LINUX is the linux GOOS value
const LINUX = "linux"

var knownGoarches = []string{AMD64, ARM64}
var knownGooses = []string{DARWIN, LINUX}

func parseSHA256Hash(hexHash string) ([]byte, error) {
	hashBytes, err := hex.DecodeString(hexHash)
	if err != nil {
		return nil, fmt.Errorf("dltools: could not decode expected hash: %w", err)
	}
	if len(hashBytes) != sha256.Size {
		return nil, fmt.Errorf("dltools: expected hash len=%d; expected %d",
			len(hashBytes), sha256.Size)
	}
	return hashBytes, nil
}

// Download saves URL as bytes in memory, verifying a SHA256 hash.
func Download(url string, expectedSHA256Hash string) ([]byte, error) {
	expectedHashBytes, err := parseSHA256Hash(expectedSHA256Hash)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := downloadUnverified(url)
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

// downloadUnverified saves URL as bytes in memory.
func downloadUnverified(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status=%s downloading url=%#v", resp.Status, url)
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
	// TODO: https://github.com/golang/go/issues/57850
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

// Platform represents the host platform as returned by Go, for use as a map key.
type Platform struct {
	GOOS   string
	GOARCH string
}

func (p Platform) String() string {
	return fmt.Sprintf("GOOS=%s GOARCH=%s", p.GOOS, p.GOARCH)
}

// GetPlatform returns the current platform from runtime.GOOS and runtime.GOARCH.
func GetPlatform() Platform {
	return Platform{runtime.GOOS, runtime.GOARCH}
}

// URLHostPlatform represents the host platform for rendering a URL template.
type URLHostPlatform struct {
	Version string
	OS      string
	Arch    string
}

// PackageFetcher downloads packages for a specific host platform.
type PackageFetcher struct {
	urlTemplate *template.Template
	hashes      map[Platform]string
	version     string
	osMap       map[string]string
	archMap     map[string]string
}

func (p *PackageFetcher) renderURL(platform Platform) (string, error) {
	os := platform.GOOS
	if p.osMap != nil {
		os = p.osMap[platform.GOOS]
		if os == "" {
			return "", fmt.Errorf("osMap missing GOOS=%s", platform.GOOS)
		}
	}
	arch := platform.GOARCH
	if p.archMap != nil {
		arch = p.archMap[platform.GOARCH]
		if arch == "" {
			return "", fmt.Errorf("archMap missing GOARCH=%s", platform.GOARCH)
		}
	}
	urlPlatform := &URLHostPlatform{
		Version: p.version,
		OS:      os,
		Arch:    arch,
	}

	buf := &bytes.Buffer{}
	err := p.urlTemplate.Execute(buf, urlPlatform)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ComputeHashes downloads the packages and computes their hashes for all Platforms.
func (p *PackageFetcher) ComputeHashes() (map[Platform]string, error) {
	out := map[Platform]string{}
	for platform := range p.hashes {
		url, err := p.renderURL(platform)
		if err != nil {
			return nil, err
		}

		urlBytes, err := downloadUnverified(url)
		if err != nil {
			return nil, err
		}

		hash := sha256.Sum256(urlBytes)
		out[platform] = hex.EncodeToString(hash[:])
	}
	return out, nil
}

// FormatHashes returns a string value of hashes to be copy/pasted into code.
func FormatHashes(hashes map[Platform]string) string {
	builder := &strings.Builder{}

	var platforms []Platform
	for platform := range hashes {
		platforms = append(platforms, platform)
	}
	sort.Slice(platforms, func(i int, j int) bool {
		if platforms[i].GOOS < platforms[j].GOOS {
			return true
		} else if platforms[i].GOOS > platforms[j].GOOS {
			return false
		}
		// GOOS equal: compare GOARCH
		if platforms[i].GOARCH < platforms[j].GOARCH {
			return true
		}
		return false
	})

	for _, platform := range platforms {
		fmt.Fprintf(builder, "\t{GOOS: %#v, GOARCH: %#v}: %#v,\n", platform.GOOS, platform.GOARCH, hashes[platform])
	}
	return builder.String()
}

// NewPackageFetcher creates a new fetcher that uses the provided urlTemplate.
func NewPackageFetcher(urlTemplate string, hashes map[Platform]string, version string) (*PackageFetcher, error) {
	parsedTemplate, err := template.New("url_template").Parse(urlTemplate)
	if err != nil {
		return nil, err
	}

	// do a "test render" with empty values to make sure the template works
	testPlatform := &URLHostPlatform{
		Version: "VVVVV",
		OS:      "OOOOO",
		Arch:    "AAAAA",
	}
	buf := &bytes.Buffer{}
	err = parsedTemplate.Execute(buf, testPlatform)
	if err != nil {
		return nil, err
	}
	if !bytes.Contains(buf.Bytes(), []byte(testPlatform.Version)) {
		return nil, fmt.Errorf("expected rendered URL to contain version; template=%#v", urlTemplate)
	}

	// check that each hash parses
	for _, hashString := range hashes {
		_, err = parseSHA256Hash(hashString)
		if err != nil {
			return nil, err
		}
	}

	return &PackageFetcher{parsedTemplate, hashes, version, nil, nil}, nil
}

// DownloadForCurrentPlatform downloads the package for the current platform.
func (p *PackageFetcher) DownloadForCurrentPlatform() ([]byte, error) {
	platform := GetPlatform()
	url, err := p.renderURL(platform)
	if err != nil {
		return nil, err
	}
	return Download(url, p.hashes[platform])
}

func sliceToSet(slice []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range slice {
		out[s] = true
	}
	return out
}

// SetOSMap configures the GOOS map for this PackageFetcher.
func (p *PackageFetcher) SetOSMap(osMap map[string]string) error {
	knownOSSet := sliceToSet(knownGooses)
	for goos := range osMap {
		if !knownOSSet[goos] {
			return fmt.Errorf("SetOSMap: map GOOS=%s not known", goos)
		}
	}
	p.osMap = osMap
	return nil
}

// SetArchMap configures the GOARCH map for this PackageFetcher.
func (p *PackageFetcher) SetArchMap(archMap map[string]string) error {
	knownArchSet := sliceToSet(knownGoarches)
	for goarch := range archMap {
		if !knownArchSet[goarch] {
			return fmt.Errorf("SetArchMap: map GOARCH=%s not known", goarch)
		}
	}
	p.archMap = archMap
	return nil
}
