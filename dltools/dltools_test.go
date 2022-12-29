package dltools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func rootHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "bad method", http.StatusMethodNotAllowed)
		return
	}

	fmt.Printf("test http server: request for Path=%s\n", req.URL.Path)
	w.Header().Set("Content-Type", "text/plain;charset=utf-8")
	switch req.URL.Path {
	case "/v1.23-osx-x64":
		w.Write([]byte("a"))
	case "/v1.23-osx-arm64":
		w.Write([]byte("b"))
	case "/v1.23-linux-x64":
		w.Write([]byte("c"))
	default:
		fmt.Printf("test http server: %s not found\n", req.URL.Path)
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func TestComputeHashes(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(rootHandler))
	defer httpServer.Close()

	urlTemplate := httpServer.URL + "/{{.Version}}-{{.OS}}-{{.Arch}}"
	hashes := map[Platform]string{
		{"darwin", "amd64"}: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
		{"darwin", "arm64"}: "3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d",
		{"linux", "amd64"}:  "2e7d2c03a9507ae265ecf5b5356885a53393a2029d241394997265a1a25aefc6",
	}
	p, err := NewPackageFetcher(urlTemplate, hashes, "v1.23")
	if err != nil {
		t.Fatal(err)
	}
	osMap := map[string]string{
		"darwin": "osx",
		"linux":  "linux",
	}
	err = p.SetOSMap(osMap)
	if err != nil {
		t.Fatal(err)
	}
	archMap := map[string]string{
		"amd64": "x64",
		"arm64": "arm64",
	}
	err = p.SetArchMap(archMap)
	if err != nil {
		t.Fatal(err)
	}

	hashes, err = p.ComputeHashes()
	if err != nil {
		panic(err)
	}

	output := FormatHashes(hashes)
	const expected = `	{GOOS: "darwin", GOARCH: "amd64"}: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
	{GOOS: "darwin", GOARCH: "arm64"}: "3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d",
	{GOOS: "linux", GOARCH: "amd64"}: "2e7d2c03a9507ae265ecf5b5356885a53393a2029d241394997265a1a25aefc6",
`
	if output != expected {
		t.Error("expected hashes:", expected)
		t.Error("computed hashes:", output)
	}
}

func TestNewPlatformFetcher(t *testing.T) {
	_, err := NewPackageFetcher("bad_template {{.DoesNotExist}}", map[Platform]string{}, "v1.23")
	if err == nil {
		t.Error("expected error for a bad template, was nil")
	}
}
