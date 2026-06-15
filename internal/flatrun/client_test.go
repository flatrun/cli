package flatrun

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDeployUsesAPIBaseAndBearerToken(t *testing.T) {
	var captured *http.Request
	var payload DeployRequest

	client := New("https://panel.example.com/api", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"ok"}`)),
		}, nil
	})

	_, err := client.Deploy(context.Background(), "my app", DeployRequest{Action: "restart", Pull: true})
	if err != nil {
		t.Fatalf("Deploy returned error: %v", err)
	}

	if captured.URL.String() != "https://panel.example.com/api/deployments/my%20app/deploy" {
		t.Fatalf("url = %s", captured.URL.String())
	}
	if captured.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("authorization = %q", captured.Header.Get("Authorization"))
	}
	if payload.Action != "restart" || !payload.Pull {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestExecuteQuickActionPostsToActionsPath(t *testing.T) {
	var captured *http.Request

	client := New("https://panel.example.com/api", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"ok"}`)),
		}, nil
	})

	if _, err := client.ExecuteQuickAction(context.Background(), "my app", "migrate"); err != nil {
		t.Fatalf("ExecuteQuickAction returned error: %v", err)
	}
	if captured.Method != http.MethodPost {
		t.Fatalf("method = %s", captured.Method)
	}
	if captured.URL.String() != "https://panel.example.com/api/deployments/my%20app/actions/migrate" {
		t.Fatalf("url = %s", captured.URL.String())
	}
	if captured.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("authorization = %q", captured.Header.Get("Authorization"))
	}
}

func TestDoTrimsTrailingAPIBaseSlash(t *testing.T) {
	var captured *http.Request

	client := New("https://panel.example.com/api/", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})

	if _, err := client.ListDeployments(context.Background()); err != nil {
		t.Fatalf("ListDeployments returned error: %v", err)
	}
	if captured.URL.String() != "https://panel.example.com/api/deployments" {
		t.Fatalf("url = %s", captured.URL.String())
	}
}

func TestCreateDeploymentPayload(t *testing.T) {
	var payload CreateDeploymentRequest

	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://panel.example.com/api/deployments" {
			t.Fatalf("url = %s", req.URL.String())
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"created"}`)),
		}, nil
	})

	_, err := client.CreateDeployment(context.Background(), CreateDeploymentRequest{
		Name:  "api",
		Image: "ghcr.io/acme/api:main",
		Ports: []PortConfig{{Container: 8080, Host: "18080"}},
	})
	if err != nil {
		t.Fatalf("CreateDeployment returned error: %v", err)
	}
	if payload.Image != "ghcr.io/acme/api:main" || payload.Ports[0].Container != 8080 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestManageUsesOperationEndpoint(t *testing.T) {
	var captured *http.Request

	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"restarted"}`)),
		}, nil
	})

	_, err := client.Manage(context.Background(), "web", "restart")
	if err != nil {
		t.Fatalf("Manage returned error: %v", err)
	}
	if captured.Method != http.MethodPost {
		t.Fatalf("method = %s", captured.Method)
	}
	if captured.URL.String() != "https://panel.example.com/api/deployments/web/restart" {
		t.Fatalf("url = %s", captured.URL.String())
	}
}

func TestPullImagesPayload(t *testing.T) {
	var payload map[string]bool

	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://panel.example.com/api/deployments/web/pull" {
			t.Fatalf("url = %s", req.URL.String())
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"pulled"}`)),
		}, nil
	})

	_, err := client.PullImages(context.Background(), "web", true)
	if err != nil {
		t.Fatalf("PullImages returned error: %v", err)
	}
	if !payload["only_latest"] {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestDeleteDeploymentQuery(t *testing.T) {
	var captured *http.Request

	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"deleted"}`)),
		}, nil
	})

	_, err := client.DeleteDeployment(context.Background(), "web", DeleteDeploymentOptions{
		DeleteSSL:      true,
		DeleteDatabase: false,
		DeleteVhost:    true,
	})
	if err != nil {
		t.Fatalf("DeleteDeployment returned error: %v", err)
	}
	if captured.Method != http.MethodDelete {
		t.Fatalf("method = %s", captured.Method)
	}
	if captured.URL.Query().Get("delete_ssl") != "true" ||
		captured.URL.Query().Get("delete_database") != "false" ||
		captured.URL.Query().Get("delete_vhost") != "true" {
		t.Fatalf("query = %s", captured.URL.RawQuery)
	}
}

func TestRawAPI(t *testing.T) {
	var captured *http.Request
	var payload map[string]string

	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})

	_, err := client.Raw(context.Background(), http.MethodPost, "/databases/list", map[string]string{"container": "mysql"})
	if err != nil {
		t.Fatalf("Raw returned error: %v", err)
	}
	if captured.URL.String() != "https://panel.example.com/api/databases/list" {
		t.Fatalf("url = %s", captured.URL.String())
	}
	if payload["container"] != "mysql" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestDeploymentImagesAndServices(t *testing.T) {
	var urls []string

	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		urls = append(urls, req.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})

	if _, err := client.DeploymentImages(context.Background(), "web"); err != nil {
		t.Fatalf("DeploymentImages returned error: %v", err)
	}
	if _, err := client.DeploymentServices(context.Background(), "web"); err != nil {
		t.Fatalf("DeploymentServices returned error: %v", err)
	}
	if _, err := client.DeploymentContainers(context.Background(), "web"); err != nil {
		t.Fatalf("DeploymentContainers returned error: %v", err)
	}

	want := []string{
		"https://panel.example.com/api/deployments/web/images",
		"https://panel.example.com/api/deployments/web/services",
		"https://panel.example.com/api/deployments/web/stats",
	}
	if strings.Join(urls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("urls = %#v, want %#v", urls, want)
	}
}

func TestDeploymentComposeEndpoints(t *testing.T) {
	var requests []string
	var updatePayload map[string]string

	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.String())
		if req.Method == http.MethodPut {
			if err := json.NewDecoder(req.Body).Decode(&updatePayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})

	if _, err := client.GetDeploymentCompose(context.Background(), "my app"); err != nil {
		t.Fatalf("GetDeploymentCompose returned error: %v", err)
	}
	if _, err := client.UpdateDeploymentCompose(context.Background(), "my app", "services: {}"); err != nil {
		t.Fatalf("UpdateDeploymentCompose returned error: %v", err)
	}

	want := []string{
		"GET https://panel.example.com/api/deployments/my%20app/compose",
		"PUT https://panel.example.com/api/deployments/my%20app",
	}
	if strings.Join(requests, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
	if updatePayload["compose_content"] != "services: {}" {
		t.Fatalf("payload = %+v", updatePayload)
	}
}

func TestDoReturnsStructuredAPIError(t *testing.T) {
	client := New("https://panel.example.com", "secret", time.Minute, false)
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
		}, nil
	})

	_, err := client.GetDeployment(context.Background(), "missing")
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", apiErr.StatusCode)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}
