package flatrun

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	HTTP    *http.Client
	Debug   io.Writer
}

type Error struct {
	StatusCode int
	Body       string
	Message    string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("flatrun API returned %d: %s", e.StatusCode, e.Message)
	}
	if e.Body == "" {
		return fmt.Sprintf("flatrun API returned %d", e.StatusCode)
	}
	return fmt.Sprintf("flatrun API returned %d: %s", e.StatusCode, e.Body)
}

func errorMessage(body string) string {
	var parsed struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err == nil {
		if parsed.Error != "" {
			return parsed.Error
		}
		if parsed.Message != "" {
			return parsed.Message
		}
	}
	return body
}

func New(baseURL, token string, timeout time.Duration, insecure bool) *Client {
	transport := http.DefaultTransport
	if insecure {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // User-controlled escape hatch for self-hosted installs.
		}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		HTTP: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// BaseURL is the agent this client talks to, which is what a cache of that agent's API
// description is keyed on.
func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) Health(ctx context.Context) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/health", nil)
}

func (c *Client) Raw(ctx context.Context, method, path string, payload any) ([]byte, error) {
	return c.Do(ctx, method, path, payload)
}

func (c *Client) ListDeployments(ctx context.Context) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/deployments", nil)
}

func (c *Client) GetDeployment(ctx context.Context, name string) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/deployments/"+url.PathEscape(name), nil)
}

func (c *Client) GetDeploymentCompose(ctx context.Context, name string) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/deployments/"+url.PathEscape(name)+"/compose", nil)
}

func (c *Client) UpdateDeploymentCompose(ctx context.Context, name, composeContent string) ([]byte, error) {
	return c.Do(ctx, http.MethodPut, "/deployments/"+url.PathEscape(name), map[string]string{
		"compose_content": composeContent,
	})
}

type DeployRequest struct {
	Action     string `json:"action"`
	Pull       bool   `json:"pull"`
	OnlyLatest bool   `json:"only_latest"`
}

func (c *Client) Deploy(ctx context.Context, name string, req DeployRequest) ([]byte, error) {
	return c.Do(ctx, http.MethodPost, "/deployments/"+url.PathEscape(name)+"/deploy", req)
}

func (c *Client) Manage(ctx context.Context, name string, operation string) ([]byte, error) {
	return c.Do(ctx, http.MethodPost, "/deployments/"+url.PathEscape(name)+"/"+url.PathEscape(operation), nil)
}

func (c *Client) ExecuteQuickAction(ctx context.Context, name, actionID string) ([]byte, error) {
	return c.Do(ctx, http.MethodPost, "/deployments/"+url.PathEscape(name)+"/actions/"+url.PathEscape(actionID), nil)
}

func (c *Client) PullImages(ctx context.Context, name string, onlyLatest bool) ([]byte, error) {
	return c.Do(ctx, http.MethodPost, "/deployments/"+url.PathEscape(name)+"/pull", map[string]bool{
		"only_latest": onlyLatest,
	})
}

func (c *Client) DeploymentImages(ctx context.Context, name string) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/deployments/"+url.PathEscape(name)+"/images", nil)
}

func (c *Client) DeploymentServices(ctx context.Context, name string) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/deployments/"+url.PathEscape(name)+"/services", nil)
}

func (c *Client) DeploymentContainers(ctx context.Context, name string) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/deployments/"+url.PathEscape(name)+"/stats", nil)
}

type CreateDeploymentRequest struct {
	Name          string       `json:"name"`
	Image         string       `json:"image,omitempty"`
	TemplateID    string       `json:"template_id,omitempty"`
	ContainerPort int          `json:"container_port,omitempty"`
	MapPorts      bool         `json:"map_ports,omitempty"`
	HostPort      string       `json:"host_port,omitempty"`
	Ports         []PortConfig `json:"ports,omitempty"`
	AutoStart     bool         `json:"auto_start,omitempty"`
}

type PortConfig struct {
	Container int    `json:"container,omitempty"`
	Host      string `json:"host,omitempty"`
}

func (c *Client) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) ([]byte, error) {
	return c.Do(ctx, http.MethodPost, "/deployments", req)
}

type DeleteDeploymentOptions struct {
	DeleteSSL      bool
	DeleteDatabase bool
	DeleteVhost    bool
}

func (c *Client) DeleteDeployment(ctx context.Context, name string, opts DeleteDeploymentOptions) ([]byte, error) {
	query := url.Values{}
	query.Set("delete_ssl", fmt.Sprintf("%t", opts.DeleteSSL))
	query.Set("delete_database", fmt.Sprintf("%t", opts.DeleteDatabase))
	query.Set("delete_vhost", fmt.Sprintf("%t", opts.DeleteVhost))
	return c.Do(ctx, http.MethodDelete, "/deployments/"+url.PathEscape(name)+"?"+query.Encode(), nil)
}

func (c *Client) ListImages(ctx context.Context) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/images", nil)
}

func (c *Client) PullImage(ctx context.Context, image, credentialID string) ([]byte, error) {
	payload := map[string]string{"name": image}
	if credentialID != "" {
		payload["credential_id"] = credentialID
	}
	return c.Do(ctx, http.MethodPost, "/images/pull", payload)
}

func (c *Client) RemoveImage(ctx context.Context, id string) ([]byte, error) {
	return c.Do(ctx, http.MethodDelete, "/images/"+url.PathEscape(id), nil)
}

func (c *Client) ListContainers(ctx context.Context) ([]byte, error) {
	return c.Do(ctx, http.MethodGet, "/containers", nil)
}

func (c *Client) ContainerOperation(ctx context.Context, id, operation string) ([]byte, error) {
	return c.Do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/"+url.PathEscape(operation), nil)
}

type ExecRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

func (c *Client) ContainerExec(ctx context.Context, id string, req ExecRequest) ([]byte, error) {
	return c.Do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/exec", req)
}

func (c *Client) RemoveContainer(ctx context.Context, id string) ([]byte, error) {
	return c.Do(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id), nil)
}

func (c *Client) Do(ctx context.Context, method, path string, payload any) ([]byte, error) {
	apiBase := strings.TrimRight(c.baseURL, "/")
	if !strings.HasSuffix(apiBase, "/api") {
		apiBase += "/api"
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.doRequest(req)
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	if c.Debug != nil {
		_, _ = fmt.Fprintf(c.Debug, "-> %s %s\n", req.Method, req.URL.String())
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if c.Debug != nil {
			_, _ = fmt.Fprintf(c.Debug, "<- error: %v\n", err)
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if c.Debug != nil {
		_, _ = fmt.Fprintf(c.Debug, "<- %d %s\n", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if c.Debug != nil {
		_, _ = fmt.Fprintf(c.Debug, "<- body %d bytes\n", len(data))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := strings.TrimSpace(string(data))
		return nil, &Error{StatusCode: resp.StatusCode, Body: body, Message: errorMessage(body)}
	}
	return data, nil
}
