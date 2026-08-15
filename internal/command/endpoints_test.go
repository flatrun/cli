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

// A program driving the CLI reads this instead of the docs.
func TestJSONListingCoversEveryEndpoint(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json"}, &stdout, &stderr); code != 0 {
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
	if len(listed) != len(catalogue()) {
		t.Fatalf("listed %d of %d commands", len(listed), len(catalogue()))
	}
	for _, e := range listed {
		if e.Family == "" || e.Op == "" || e.Method == "" || !strings.HasPrefix(e.Path, "/") {
			t.Fatalf("incomplete entry: %+v", e)
		}
	}
}

func TestJSONListingNarrowsToOneFamily(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"backups", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}

	var listed []struct {
		Family string `json:"family"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatalf("the listing must be valid JSON: %v", err)
	}
	if len(listed) == 0 {
		t.Fatal("no commands listed")
	}
	for _, e := range listed {
		if e.Family != "backups" {
			t.Fatalf("asked for one family, got %s", e.Family)
		}
	}
}

// A caller reading the JSON must see the hand-shaped commands too, or it only learns half of
// what the CLI can do and reaches for the raw endpoint instead.
func TestJSONListingIncludesHandShapedCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}

	var listed []struct {
		Family  string `json:"family"`
		Op      string `json:"op"`
		Command string `json:"command"`
		Shaped  bool   `json:"shaped"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range listed {
		if e.Family == "deployment" && e.Op == "exec" {
			found = true
			if !e.Shaped {
				t.Error("a hand-shaped command should say so")
			}
			if !strings.Contains(e.Command, "-- COMMAND") {
				t.Errorf("the invocation should show what it takes, got %q", e.Command)
			}
		}
	}
	if !found {
		t.Error("deployment exec is missing from the listing")
	}
}

// Both names for one resource reach the same operations.
func TestSingularFamilyListsWhatThePluralDoes(t *testing.T) {
	var singular, plural, stderr bytes.Buffer
	if code := Run([]string{"deployment", "--json"}, &singular, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if code := Run([]string{"deployments", "--json"}, &plural, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}

	ops := func(raw []byte) map[string]bool {
		var listed []struct {
			Op string `json:"op"`
		}
		if err := json.Unmarshal(raw, &listed); err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, e := range listed {
			out[e.Op] = true
		}
		return out
	}

	for op := range ops(plural.Bytes()) {
		if !ops(singular.Bytes())[op] {
			t.Errorf("deployments %s is not reachable as deployment %s", op, op)
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
		// A parameter is written :name, or *name when it holds the rest of the path.
		params := strings.Count(e.path, ":") + strings.Count(e.path, "*")
		if params != len(e.args) {
			t.Errorf("%s has %d path parameters but %d arguments", e.path, params, len(e.args))
		}
	}
}

// A wildcard holds the rest of the path, so its separators are structure and must survive.
func TestWildcardArgumentKeepsItsSeparators(t *testing.T) {
	server, got := recordingServer(t, `{}`)

	code, _, stderr := runCLI(t, server, "deployments", "files-get", "shop", "src/app/main.go", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.path != "/api/deployments/shop/files/src/app/main.go" {
		t.Fatalf("path = %s", got.path)
	}
}

func TestWildcardArgumentStillEscapesEachSegment(t *testing.T) {
	server, got := recordingServer(t, `{}`)

	code, _, stderr := runCLI(t, server, "deployments", "files-get", "shop", "a b/c?d", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(got.rawURI, "a%20b/c%3Fd") {
		t.Fatalf("the segments were not escaped: %s", got.rawURI)
	}
}

// Nobody should have to remember whether the API called it backup or backups.
func TestEitherSpellingReachesTheSameResource(t *testing.T) {
	for _, spelling := range []string{"backups", "backup", "certificates", "certificate"} {
		server, got := recordingServer(t, `{"items":[],"total":0}`)
		code, _, stderr := runCLI(t, server, spelling, "list", "--json")
		if code != 0 {
			t.Fatalf("%s: code=%d stderr=%s", spelling, code, stderr)
		}
		if got.path == "" {
			t.Errorf("%s reached nothing", spelling)
		}
	}
}

func TestUnknownResourceIsStillUnknown(t *testing.T) {
	server, got := recordingServer(t, `{}`)
	code, _, stderr := runCLI(t, server, "bakcups", "list")
	if code == 0 {
		t.Fatal("a misspelt resource should not be accepted")
	}
	if got.path != "" {
		t.Fatal("nothing should have been sent")
	}
	if !strings.Contains(stderr, "Unknown command") {
		t.Fatalf("stderr = %s", stderr)
	}
}

// Whatever a command printed before, it prints now: a pipe is not a reason to change the answer,
// and `deployment list | grep running` has to keep working.
func TestOutputDoesNotChangeWhenPiped(t *testing.T) {
	server, _ := recordingServer(t, `{"containers":[{"id":"c-1","name":"web","state":"running"}]}`)
	code, stdout, stderr := runCLI(t, server, "container", "list")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "CONTAINER ID") {
		t.Fatalf("expected the table, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "web") {
		t.Fatalf("expected the row, got:\n%s", stdout)
	}
}

func TestJSONIsHowYouAskForTheAnswer(t *testing.T) {
	server, _ := recordingServer(t, `{"containers":[{"id":"c-1","name":"web","state":"running"}]}`)
	code, stdout, stderr := runCLI(t, server, "container", "list", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("--json should print the raw answer, got:\n%s", stdout)
	}
}

// The exact payload a new agent sends for a deployment list: the shared shape, the name it used
// to answer under, and the field it answered alongside. A CLI built before any of that has to
// keep reading it.
const newAgentDeploymentList = `{"deployments":[{"name":"trakli-staging","path":"/opt/flatrun/deployments/trakli-staging","status":"running","created_at":"2026-08-13T22:39:28Z","updated_at":"0001-01-01T00:00:00Z"}],"items":[{"name":"trakli-staging","path":"/opt/flatrun/deployments/trakli-staging","status":"running","created_at":"2026-08-13T22:39:28Z","updated_at":"0001-01-01T00:00:00Z"}],"path":"/opt/flatrun/deployments","total":1}`

func TestOldCommandsReadANewAgentsAnswer(t *testing.T) {
	for _, command := range [][]string{{"deployment", "list"}, {"deployments", "list"}} {
		server, _ := recordingServer(t, newAgentDeploymentList)
		code, stdout, stderr := runCLI(t, server, command...)
		if code != 0 {
			t.Fatalf("%v: code=%d stderr=%s", command, code, stderr)
		}
		if !strings.Contains(stdout, "NAME") || !strings.Contains(stdout, "trakli-staging") {
			t.Fatalf("%v: expected the table, got:\n%s", command, stdout)
		}
		// The rows must come from one of the two names, not from both.
		if strings.Count(stdout, "trakli-staging") != 1 {
			t.Fatalf("%v: the deployment was listed twice:\n%s", command, stdout)
		}
	}
}
