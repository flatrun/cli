package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"0.4.0", "0.3.0", true},
		{"0.4.0-beta.5", "0.4.0-beta.4", true},
		{"0.4.0-beta.4", "0.4.0-beta.4", false},
		{"0.3.0", "0.4.0-beta.4", false},
		{"0.4.0-beta.4", "0.4.0", false},
		{"0.4.0-beta.10", "0.4.0-beta.4", true},
	}
	for _, tt := range tests {
		t.Run(tt.candidate+"_from_"+tt.current, func(t *testing.T) {
			got, err := newer(tt.candidate, tt.current)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("newer(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
			}
		})
	}
}

func TestRunDownloadsVerifiesAndReplacesExecutable(t *testing.T) {
	archiveName := fmt.Sprintf("flatrun-0.4.0-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binaryName := fmt.Sprintf("flatrun-0.4.0-%s-%s", runtime.GOOS, runtime.GOARCH)
	archive := testArchive(t, binaryName, []byte("new binary"))
	sum := sha256.Sum256(archive)
	checksums := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			_ = json.NewEncoder(w).Encode(release{
				TagName: "v0.4.0",
				Assets: []asset{
					{Name: archiveName, URL: "http://" + r.Host + "/archive"},
					{Name: "checksums.txt", URL: "http://" + r.Host + "/checksums"},
				},
			})
		case "/archive":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldReleasesURL := releasesURL
	oldExecutable := executable
	releasesURL = server.URL + "/releases"
	path := filepath.Join(t.TempDir(), "flatrun")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return path, nil }
	t.Cleanup(func() {
		releasesURL = oldReleasesURL
		executable = oldExecutable
	})

	result, err := Run(context.Background(), "0.3.0", false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || result.Latest != "0.4.0" {
		t.Fatalf("result = %+v", result)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary" {
		t.Fatalf("executable = %q", got)
	}
}

func testArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
