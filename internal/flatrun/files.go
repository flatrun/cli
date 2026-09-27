package flatrun

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (c *Client) PushDeploymentFiles(ctx context.Context, deployment, source, destination string, deleteMissing bool) ([]byte, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source must be a directory")
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	errCh := make(chan error, 1)
	go func() {
		errCh <- writePushBody(multipartWriter, writer, source, destination, deleteMissing)
	}()

	apiBase := strings.TrimRight(c.baseURL, "/")
	if !strings.HasSuffix(apiBase, "/api") {
		apiBase += "/api"
	}
	path := "/deployments/" + url.PathEscape(deployment) + "/files-push"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+path, reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	data, requestErr := c.doRequest(req)
	archiveErr := <-errCh
	if requestErr != nil {
		return nil, requestErr
	}
	if archiveErr != nil {
		return nil, archiveErr
	}
	return data, nil
}

func (c *Client) PullDeploymentFile(ctx context.Context, deployment, source, destination string) ([]byte, error) {
	apiBase := strings.TrimRight(c.baseURL, "/")
	if !strings.HasSuffix(apiBase, "/api") {
		apiBase += "/api"
	}
	path := "/deployments/" + url.PathEscape(deployment) + "/files/" + escapeFilePath(source)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return nil, readErr
		}
		body := strings.TrimSpace(string(data))
		return nil, &Error{StatusCode: resp.StatusCode, Body: body, Message: errorMessage(body)}
	}
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(dir, ".flatrun-download-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	written, err := io.Copy(tmp, resp.Body)
	if err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"source":%q,"destination":%q,"bytes":%d}`, source, destination, written)), nil
}

func escapeFilePath(path string) string {
	parts := strings.Split(strings.TrimPrefix(filepath.ToSlash(path), "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func writePushBody(multipartWriter *multipart.Writer, pipe *io.PipeWriter, source, destination string, deleteMissing bool) error {
	fail := func(err error) error {
		_ = pipe.CloseWithError(err)
		return err
	}
	if err := multipartWriter.WriteField("destination", destination); err != nil {
		return fail(err)
	}
	if err := multipartWriter.WriteField("delete", strconv.FormatBool(deleteMissing)); err != nil {
		return fail(err)
	}
	part, err := multipartWriter.CreateFormFile("archive", "content.tar.gz")
	if err != nil {
		return fail(err)
	}
	gzipWriter := gzip.NewWriter(part)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := writeDirectoryArchive(tarWriter, source); err != nil {
		return fail(err)
	}
	if err := tarWriter.Close(); err != nil {
		return fail(err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fail(err)
	}
	if err := multipartWriter.Close(); err != nil {
		return fail(err)
	}
	return pipe.Close()
}

func writeDirectoryArchive(writer *tar.Writer, source string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if entry.IsDir() {
			header.Name += "/"
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
