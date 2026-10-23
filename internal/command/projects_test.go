package command

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A slice of the description an agent gives once it names deployments projects.
const projectsSpec = `{
  "openapi": "3.1.0",
  "info": {"version": "0.6.0"},
  "paths": {
    "/api/projects": {
      "post": {
        "operationId": "post-projects",
        "requestBody": {"required": true, "content": {"application/json": {
          "schema": {"type": "object", "required": ["name"], "properties": {
            "name": {"type": "string"}, "image": {"type": "string"}, "compose_content": {"type": "string"}
          }}
        }}},
        "responses": {"201": {"description": "Success"}}
      }
    }
  }
}`

func projectsServer(t *testing.T) (*httptest.Server, *recordedRequest) {
	t.Helper()
	got := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			_, _ = w.Write([]byte(projectsSpec))
			return
		}
		got.method = r.Method
		got.path = r.URL.Path
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	t.Cleanup(server.Close)
	return server, got
}

func TestDeploymentCommandsReadAProjectsDescription(t *testing.T) {
	isolateCache(t)
	server, got := projectsServer(t)

	code, stdout, stderr := runCLI(t, server, "deployments", "create", "--generate-cli-skeleton")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(stdout), &body); err != nil {
		t.Fatalf("skeleton is not JSON: %v: %s", err, stdout)
	}
	if _, ok := body["compose_content"]; !ok {
		t.Fatalf("skeleton = %#v", body)
	}

	if code, _, _ := runCLI(t, server, "deployments", "create", "-f", "nmae=shop"); code == 0 {
		t.Fatal("a field the project endpoint does not take should fail")
	}
	if got.path != "" {
		t.Fatalf("nothing should have been sent, but %s %s was", got.method, got.path)
	}
}

// Older agents serve only the deployment paths, so a project command still calls those.
func TestProjectsIsAnotherNameForDeployments(t *testing.T) {
	isolateCache(t)
	server, got := projectsServer(t)

	for _, family := range []string{"projects", "project"} {
		got.path = ""
		code, _, stderr := runCLI(t, server, family, "list", "--json")
		if code != 0 {
			t.Fatalf("%s list: code=%d stderr=%s", family, code, stderr)
		}
		if got.method != http.MethodGet || got.path != "/api/deployments" {
			t.Fatalf("%s list called %s %s", family, got.method, got.path)
		}
	}
}

func TestProjectDeploymentsListDeployHistory(t *testing.T) {
	isolateCache(t)
	server, got := projectsServer(t)

	code, _, stderr := runCLI(t, server, "project", "deployments", "shop", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if got.method != http.MethodGet || got.path != "/api/deployments/shop/deployments" {
		t.Fatalf("called %s %s", got.method, got.path)
	}
	if code, _, stderr := runCLI(t, server, "project", "deployments-logs", "shop", "d1", "--json"); code != 0 || got.path != "/api/deployments/shop/deployments/d1/logs" {
		t.Fatalf("logs: code=%d path=%s stderr=%s", code, got.path, stderr)
	}
}
