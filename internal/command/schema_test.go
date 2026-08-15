package command

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A slice of a real agent's description: one endpoint with a required field, and one that
// answers with rows and says which of their fields make columns.
const testSpec = `{
  "openapi": "3.1.0",
  "info": {"version": "0.4.0"},
  "paths": {
    "/api/backups": {
      "post": {
        "operationId": "post-backups",
        "x-permission": "backups:write",
        "requestBody": {"required": true, "content": {"application/json": {
          "schema": {"$ref": "#/components/schemas/backup.CreateBackupRequest"}}}},
        "responses": {"200": {"description": "Success"}}
      },
      "get": {
        "operationId": "get-backups",
        "parameters": [{"name": "deployment", "in": "query", "schema": {"type": "string"}}],
        "responses": {"200": {"description": "Success", "content": {"application/json": {
          "schema": {"$ref": "#/components/schemas/api.ListOfBackup"}}}}}
      }
    }
  },
  "components": {"schemas": {
    "backup.CreateBackupRequest": {
      "type": "object",
      "required": ["deployment_name"],
      "x-property-order": ["deployment_name", "description"],
      "properties": {
        "deployment_name": {"type": "string"},
        "description": {"type": "string"}
      }
    },
    "api.ListOfBackup": {
      "type": "object",
      "x-render": "list",
      "x-property-order": ["items", "total"],
      "properties": {
        "items": {"type": "array", "items": {"$ref": "#/components/schemas/backup.Backup"}},
        "total": {"type": "integer"}
      }
    },
    "backup.Backup": {
      "type": "object",
      "x-columns": ["id", "deployment_name", "status"],
      "x-property-order": ["id", "deployment_name", "status", "path"],
      "properties": {
        "id": {"type": "string"},
        "deployment_name": {"type": "string"},
        "status": {"type": "string"},
        "path": {"type": "string"}
      }
    }
  }}
}`

// describingServer answers the description on /openapi.json and the given reply everywhere else.
func describingServer(t *testing.T, reply string) (*httptest.Server, *recordedRequest) {
	t.Helper()
	got := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			_, _ = w.Write([]byte(testSpec))
			return
		}
		got.method = r.Method
		got.path = r.URL.Path
		got.query = r.URL.RawQuery
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)
	return server, got
}

// Each test gets its own cache, or one test's description would answer another's question.
func isolateCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", filepath.Join(dir, "home"))
	if err := os.MkdirAll(filepath.Join(dir, "home"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownFieldIsRefusedBeforeSending(t *testing.T) {
	isolateCache(t)
	server, got := describingServer(t, `{"message":"created"}`)

	code, _, stderr := runCLI(t, server, "backups", "create", "-f", "deployment_nmae=shop")
	if code == 0 {
		t.Fatal("a field the endpoint does not take should fail")
	}
	if got.path != "" {
		t.Fatalf("nothing should have been sent, but %s %s was", got.method, got.path)
	}
	if !strings.Contains(stderr, "deployment_name") {
		t.Fatalf("the error should name the field that was meant, got %s", stderr)
	}
}

func TestKnownFieldsAreSent(t *testing.T) {
	isolateCache(t)
	server, got := describingServer(t, `{"message":"created"}`)

	code, _, stderr := runCLI(t, server, "backups", "create", "-f", "deployment_name=shop", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.path != "/api/backups" {
		t.Fatalf("path = %s", got.path)
	}
}

func TestUnknownQueryParameterIsRefused(t *testing.T) {
	isolateCache(t)
	server, got := describingServer(t, `{"backups":[]}`)

	code, _, stderr := runCLI(t, server, "backups", "list", "-q", "deploymnet=shop")
	if code == 0 {
		t.Fatal("a query parameter the endpoint does not read should fail")
	}
	if got.path != "" {
		t.Fatal("nothing should have been sent")
	}
	if !strings.Contains(stderr, "deployment") {
		t.Fatalf("the error should name the parameter that was meant, got %s", stderr)
	}
}

// The layout comes from the description, so an endpoint nobody wrote a renderer for still prints
// as a table.
func TestAnswerIsRenderedFromTheDescription(t *testing.T) {
	isolateCache(t)
	reply := `{"items":[
      {"id":"b-1","deployment_name":"shop","status":"complete","path":"/srv/b-1"},
      {"id":"b-2","deployment_name":"blog","status":"failed","path":"/srv/b-2"}],"total":2,"backups":[
      {"id":"b-1","deployment_name":"shop","status":"complete","path":"/srv/b-1"},
      {"id":"b-2","deployment_name":"blog","status":"failed","path":"/srv/b-2"}]}`
	server, _ := describingServer(t, reply)

	code, stdout, stderr := runCLI(t, server, "backups", "list")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "DEPLOYMENT NAME") {
		t.Fatalf("expected column headings, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "b-1") || !strings.Contains(stdout, "complete") {
		t.Fatalf("expected the rows, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "/srv/b-1") {
		t.Fatalf("a field that is not a column should stay out of the table:\n%s", stdout)
	}
}

func TestJSONStillWinsOverTheTable(t *testing.T) {
	isolateCache(t)
	server, _ := describingServer(t, `{"items":[{"id":"b-1","deployment_name":"shop","status":"complete"}],"total":1}`)

	code, stdout, stderr := runCLI(t, server, "backups", "list", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("--json should print the raw answer, got:\n%s", stdout)
	}
}

func TestHelpShowsWhatAnEndpointTakes(t *testing.T) {
	isolateCache(t)
	server, _ := describingServer(t, `{}`)

	code, stdout, stderr := runCLI(t, server, "backups", "create", "--help")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, want := range []string{"deployment_name", "required", "description", "backups:write"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help should mention %q, got:\n%s", want, stdout)
		}
	}
}

// An older agent that cannot describe itself still has to work.
func TestCommandsWorkWithoutADescription(t *testing.T) {
	isolateCache(t)
	got := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		got.path = r.URL.Path
		_, _ = w.Write([]byte(`{"backups":[]}`))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")
	if code := Run([]string{"backups", "list", "-q", "anything=goes"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.path != "/api/backups" {
		t.Fatalf("path = %s", got.path)
	}
}

// A column heading over one column of names is furniture, so names print as names.
func TestNamesPrintAsLinesNotATable(t *testing.T) {
	isolateCache(t)
	spec := strings.Replace(testSpec,
		`"x-columns": ["id", "deployment_name", "status"]`,
		`"x-columns": ["id"]`, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			_, _ = w.Write([]byte(spec))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"b-1"},{"id":"b-2"}],"total":2}`))
	}))
	defer server.Close()

	code, stdout, stderr := runCLI(t, server, "backups", "list")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.Contains(stdout, "ID") {
		t.Fatalf("one column needs no heading, got:\n%s", stdout)
	}
	if stdout != "b-1\nb-2\n" {
		t.Fatalf("expected one name per line, got:\n%q", stdout)
	}
}

// A required field is required however the body was given, so --data is held to it too.
func TestRequiredFieldIsCheckedWhateverCarriesTheBody(t *testing.T) {
	isolateCache(t)
	server, got := describingServer(t, `{"message":"created"}`)

	code, _, stderr := runCLI(t, server, "backups", "create", "--data", `{"description":"nightly"}`)
	if code == 0 {
		t.Fatal("a body missing a required field should fail")
	}
	if got.path != "" {
		t.Fatal("nothing should have been sent")
	}
	if !strings.Contains(stderr, "deployment_name") {
		t.Fatalf("the error should name the missing field, got %s", stderr)
	}
}

func TestNoBodyAtAllStillReportsWhatIsRequired(t *testing.T) {
	isolateCache(t)
	server, _ := describingServer(t, `{"message":"created"}`)

	code, _, stderr := runCLI(t, server, "backups", "create")
	if code == 0 {
		t.Fatal("an endpoint with a required field should not accept an empty body")
	}
	if !strings.Contains(stderr, "deployment_name") {
		t.Fatalf("the error should name the missing field, got %s", stderr)
	}
}

// An agent that has not typed an endpoint, or is too old to describe itself at all, should still
// get a table rather than a wall of JSON.
func TestUndescribedListStillPrintsAsATable(t *testing.T) {
	isolateCache(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"backups":[{"id":"b-1","deployment_name":"shop","status":"complete"}]}`))
	}))
	defer server.Close()

	code, stdout, stderr := runCLI(t, server, "backups", "list")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "b-1") {
		t.Fatalf("expected a table, got:\n%s", stdout)
	}
}

// Columns follow the order the answer wrote them in, since alphabetical buries the identifier.
func TestInferredColumnsKeepTheAnswersOrder(t *testing.T) {
	isolateCache(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"backups":[{"id":"b-1","zone":"eu","alpha":"a"}]}`))
	}))
	defer server.Close()

	_, stdout, _ := runCLI(t, server, "backups", "list")
	heading := strings.SplitN(stdout, "\n", 2)[0]
	if strings.Index(heading, "ID") > strings.Index(heading, "ALPHA") {
		t.Fatalf("the identifier should come first, got %q", heading)
	}
}
