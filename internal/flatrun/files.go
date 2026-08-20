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
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported: %s", path)
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
