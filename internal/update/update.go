package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const maxDownloadSize = 100 << 20

var (
	releasesURL = "https://api.github.com/repos/flatrun/cli/releases"
	executable  = os.Executable
)

type release struct {
	TagName string  `json:"tag_name"`
	Draft   bool    `json:"draft"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Result struct {
	Current string
	Latest  string
	Updated bool
}

func Run(ctx context.Context, current string, checkOnly bool) (Result, error) {
	result := Result{Current: current}
	if current == "dev" {
		return result, errors.New("development builds cannot be updated automatically")
	}

	rel, err := latest(ctx, strings.Contains(current, "-"))
	if err != nil {
		return result, err
	}
	result.Latest = strings.TrimPrefix(rel.TagName, "v")
	isNewer, err := newer(result.Latest, current)
	if err != nil {
		return result, err
	}
	if !isNewer || checkOnly {
		return result, nil
	}

	archiveName := fmt.Sprintf("flatrun-%s-%s-%s.tar.gz", result.Latest, runtime.GOOS, runtime.GOARCH)
	archiveAsset, ok := findAsset(rel.Assets, archiveName)
	if !ok {
		return result, fmt.Errorf("release %s does not provide %s/%s", result.Latest, runtime.GOOS, runtime.GOARCH)
	}
	checksumsAsset, ok := findAsset(rel.Assets, "checksums.txt")
	if !ok {
		return result, errors.New("release does not provide checksums.txt")
	}

	archive, err := download(ctx, archiveAsset.URL)
	if err != nil {
		return result, fmt.Errorf("download release: %w", err)
	}
	checksums, err := download(ctx, checksumsAsset.URL)
	if err != nil {
		return result, fmt.Errorf("download checksums: %w", err)
	}
	if err := verify(archiveName, archive, string(checksums)); err != nil {
		return result, err
	}
	binaryName := fmt.Sprintf("flatrun-%s-%s-%s", result.Latest, runtime.GOOS, runtime.GOARCH)
	binary, err := extract(archive, binaryName)
	if err != nil {
		return result, err
	}
	if err := replace(binary); err != nil {
		return result, err
	}
	result.Updated = true
	return result, nil
}

func latest(ctx context.Context, includePrerelease bool) (release, error) {
	url := releasesURL + "/latest"
	if includePrerelease {
		url = releasesURL + "?per_page=20"
	}
	body, err := download(ctx, url)
	if err != nil {
		return release{}, fmt.Errorf("check latest release: %w", err)
	}
	if !includePrerelease {
		var rel release
		if err := json.Unmarshal(body, &rel); err != nil {
			return release{}, fmt.Errorf("read latest release: %w", err)
		}
		return rel, nil
	}
	var releases []release
	if err := json.Unmarshal(body, &releases); err != nil {
		return release{}, fmt.Errorf("read releases: %w", err)
	}
	for _, rel := range releases {
		if !rel.Draft {
			return rel, nil
		}
	}
	return release{}, errors.New("no published release found")
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "flatrun-cli")
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownloadSize {
		return nil, errors.New("download exceeds 100 MiB limit")
	}
	return data, nil
}

func findAsset(assets []asset, name string) (asset, bool) {
	for _, candidate := range assets {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return asset{}, false
}

func verify(name string, archive []byte, checksums string) error {
	want := ""
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksum for %s is missing", name)
	}
	sum := sha256.Sum256(archive)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), want) {
		return errors.New("release checksum does not match")
	}
	return nil
}

func extract(archive []byte, binaryName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		if header.Name != binaryName || header.Typeflag != tar.TypeReg {
			continue
		}
		if header.Size < 0 || header.Size > maxDownloadSize {
			return nil, errors.New("release binary exceeds 100 MiB limit")
		}
		binary, err := io.ReadAll(io.LimitReader(tr, header.Size))
		if err != nil {
			return nil, fmt.Errorf("read release binary: %w", err)
		}
		if int64(len(binary)) != header.Size {
			return nil, errors.New("release binary is truncated")
		}
		return binary, nil
	}
	return nil, fmt.Errorf("release archive does not contain %s", binaryName)
}

func replace(binary []byte) error {
	path, err := executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect current executable: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".flatrun-update-*")
	if err != nil {
		return fmt.Errorf("prepare update beside %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set update permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync update: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close update: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func newer(candidate, current string) (bool, error) {
	candidateParts, err := parseVersion(candidate)
	if err != nil {
		return false, fmt.Errorf("latest release: %w", err)
	}
	currentParts, err := parseVersion(current)
	if err != nil {
		return false, fmt.Errorf("current version: %w", err)
	}
	for i := 0; i < 3; i++ {
		if candidateParts.numbers[i] != currentParts.numbers[i] {
			return candidateParts.numbers[i] > currentParts.numbers[i], nil
		}
	}
	if candidateParts.pre == currentParts.pre {
		return false, nil
	}
	if candidateParts.pre == "" {
		return true, nil
	}
	if currentParts.pre == "" {
		return false, nil
	}
	return comparePrerelease(candidateParts.pre, currentParts.pre) > 0, nil
}

type versionParts struct {
	numbers [3]int
	pre     string
}

func parseVersion(value string) (versionParts, error) {
	value = strings.TrimPrefix(value, "v")
	value = strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(value, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return versionParts{}, fmt.Errorf("invalid version %q", value)
	}
	parsed := versionParts{}
	for i, part := range core {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return versionParts{}, fmt.Errorf("invalid version %q", value)
		}
		parsed.numbers[i] = n
	}
	if len(parts) == 2 {
		parsed.pre = parts[1]
	}
	return parsed, nil
}

func comparePrerelease(left, right string) int {
	l := strings.FieldsFunc(left, func(r rune) bool { return r == '.' || r == '-' })
	r := strings.FieldsFunc(right, func(r rune) bool { return r == '.' || r == '-' })
	for i := 0; i < len(l) && i < len(r); i++ {
		ln, le := strconv.Atoi(l[i])
		rn, re := strconv.Atoi(r[i])
		if le == nil && re == nil && ln != rn {
			if ln > rn {
				return 1
			}
			return -1
		}
		if l[i] != r[i] {
			if le == nil {
				return -1
			}
			if re == nil || l[i] > r[i] {
				return 1
			}
			return -1
		}
	}
	if len(l) > len(r) {
		return 1
	}
	if len(l) < len(r) {
		return -1
	}
	return 0
}
