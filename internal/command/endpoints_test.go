package command

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordedRequest struct {
	method string
	path   string
	query  string
	rawURI string
	body   map[string]any
}

func recordingServer(t *testing.T, reply string) (*httptest.Server, *recordedRequest) {
	t.Helper()
	got := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.query = r.URL.RawQuery
		got.rawURI = r.RequestURI
		if raw, err := io.ReadAll(r.Body); err == nil && len(raw) > 0 {
			_ = json.Unmarshal(raw, &got.body)
		}
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)
	return server, got
}

func runCLI(t *testing.T, server *httptest.Server, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// The whole point of the generated table is that an agent endpoint is reachable without a
// hand-written wrapper, so this drives one that has none.
func TestGeneratedCommandCallsTheEndpoint(t *testing.T) {
	server, got := recordingServer(t, `{"backups":[]}`)

	code, _, stderr := runCLI(t, server, "backups", "list", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.method != http.MethodGet || got.path != "/api/backups" {
		t.Fatalf("called %s %s", got.method, got.path)
	}
}

func TestGeneratedCommandSubstitutesPathArguments(t *testing.T) {
	server, got := recordingServer(t, `{"message":"ok"}`)

	code, _, stderr := runCLI(t, server, "certificates", "renew", "shop.example.com", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.path != "/api/certificates/shop.example.com/renew" {
		t.Fatalf("path = %s", got.path)
	}
	if got.method != http.MethodPost {
		t.Fatalf("method = %s", got.method)
	}
}

// A domain with a slash or a space in it must not be able to reshape the request path.
func TestGeneratedCommandEscapesPathArguments(t *testing.T) {
	server, got := recordingServer(t, `{}`)

	code, _, stderr := runCLI(t, server, "certificates", "get", "one/../../admin", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	// The server decodes before it hands over URL.Path, so what matters is what went on the
	// wire: one escaped segment rather than a walk up the tree.
	if !strings.Contains(got.rawURI, "%2F") {
		t.Fatalf("the argument was not escaped on the wire: %s", got.rawURI)
	}
}

func TestGeneratedCommandBuildsABodyFromFields(t *testing.T) {
	server, got := recordingServer(t, `{"message":"saved"}`)

	code, _, stderr := runCLI(t, server, "settings", "update", "-f", "name=backups", "-f", "enabled=true", "-f", "retention=7", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.body["name"] != "backups" {
		t.Errorf("name = %#v", got.body["name"])
	}
	if got.body["enabled"] != true {
		t.Errorf("enabled should be sent as a boolean, got %#v", got.body["enabled"])
	}
	if got.body["retention"] != float64(7) {
		t.Errorf("retention should be sent as a number, got %#v", got.body["retention"])
	}
}

func TestGeneratedCommandPassesQueryParameters(t *testing.T) {
	server, got := recordingServer(t, `{"logs":""}`)

	code, _, stderr := runCLI(t, server, "deployments", "logs", "shop", "-q", "service=web", "-q", "tail=50", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.path != "/api/deployments/shop/logs" {
		t.Fatalf("path = %s", got.path)
	}
	if got.query != "service=web&tail=50" {
		t.Fatalf("query = %s", got.query)
	}
}

func TestGeneratedCommandRejectsBothBodyForms(t *testing.T) {
	server, _ := recordingServer(t, `{}`)

	code, _, stderr := runCLI(t, server, "settings", "update", "--data", `{"a":1}`, "-f", "b=2")
	if code == 0 {
		t.Fatal("sending a body two ways at once should fail")
	}
	if !strings.Contains(stderr, "not both") {
		t.Fatalf("stderr = %s", stderr)
	}
}

func TestMissingArgumentIsRefusedBeforeAnyRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()

	code, _, stderr := runCLI(t, server, "certificates", "renew")
	if code == 0 {
		t.Fatal("a missing argument should not be accepted")
	}
	if called {
		t.Fatal("nothing should have been sent")
	}
	if !strings.Contains(stderr, "flatrun certificates renew DOMAIN") {
		t.Fatalf("the usage should name the argument, got %s", stderr)
	}
}

// The singular family is hand-shaped and the plural one is generated; an operator should not
// have to know which is which.
func TestHandWrittenFamilyFallsBackToTheTable(t *testing.T) {
	server, got := recordingServer(t, `{"sources":[]}`)

	code, _, stderr := runCLI(t, server, "deployment", "log-sources", "shop", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.path != "/api/deployments/shop/log-sources" {
		t.Fatalf("path = %s", got.path)
	}
}

// An agent driving the CLI reads this instead of the docs.
func TestCommandsListsEveryEndpointAsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"commands", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}

	var listed []struct {
		Family  string   `json:"family"`
		Op      string   `json:"op"`
		Method  string   `json:"method"`
		Path    string   `json:"path"`
		Args    []string `json:"args"`
		Command string   `json:"command"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatalf("the listing must be valid JSON: %v", err)
	}
	if len(listed) != len(generatedEndpoints) {
		t.Fatalf("listed %d of %d endpoints", len(listed), len(generatedEndpoints))
	}
	for _, e := range listed {
		if e.Family == "" || e.Op == "" || e.Method == "" || !strings.HasPrefix(e.Path, "/") {
			t.Fatalf("incomplete entry: %+v", e)
		}
	}
}

func TestEveryGeneratedCommandIsReachableAndUnique(t *testing.T) {
	seen := map[string]string{}
	for _, e := range generatedEndpoints {
		key := e.family + " " + e.op
		if previous, clash := seen[key]; clash {
			t.Errorf("%q maps to both %s and %s", key, previous, e.path)
		}
		seen[key] = e.path

		found, ok := findEndpoint(e.family, e.op)
		if !ok || found.path != e.path {
			t.Errorf("%q does not dispatch back to %s", key, e.path)
		}
		if strings.Count(e.path, ":") != len(e.args) {
			t.Errorf("%s has %d path parameters but %d arguments", e.path, strings.Count(e.path, ":"), len(e.args))
		}
	}
}
