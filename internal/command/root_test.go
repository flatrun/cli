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

func TestConfigureSetAndList(t *testing.T) {
	t.Setenv("FLATRUN_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"configure", "set", "--profile", "prod", "--url", "https://panel.example.com", "--token", "secret"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("configure set code=%d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"configure", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("configure list code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "* prod\thttps://panel.example.com") {
		t.Fatalf("unexpected list output: %s", stdout.String())
	}
}

func TestConfigureSetReadsTokenFromStdin(t *testing.T) {
	t.Setenv("FLATRUN_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	oldStdin := stdin
	oldStdinIsTerminal := stdinIsTerminal
	stdin = strings.NewReader("secret-from-stdin\n")
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() {
		stdin = oldStdin
		stdinIsTerminal = oldStdinIsTerminal
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"configure", "set", "--url", "https://panel.example.com", "--token-stdin"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"configure", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "https://panel.example.com") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestConfigureSetRejectsTokenAndTokenStdinTogether(t *testing.T) {
	t.Setenv("FLATRUN_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	oldStdin := stdin
	oldStdinIsTerminal := stdinIsTerminal
	stdin = strings.NewReader("stdin-secret\n")
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() {
		stdin = oldStdin
		stdinIsTerminal = oldStdinIsTerminal
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"configure", "set", "--url", "https://panel.example.com", "--token", "flag-secret", "--token-stdin"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestConfigureSetRejectsTokenStdinFromTerminal(t *testing.T) {
	t.Setenv("FLATRUN_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"configure", "set", "--url", "https://panel.example.com", "--token-stdin"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "requires piped input") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestConfigureUseSwitchesCurrentProfile(t *testing.T) {
	t.Setenv("FLATRUN_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"configure", "set", "--profile", "prod", "--url", "https://prod.example", "--token", "prod-token"}, &stdout, &stderr); code != 0 {
		t.Fatalf("set prod code=%d stderr=%s", code, stderr.String())
	}
	if code := Run([]string{"configure", "set", "--profile", "staging", "--url", "https://staging.example", "--token", "staging-token"}, &stdout, &stderr); code != 0 {
		t.Fatalf("set staging code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	code := Run([]string{"configure", "use", "prod"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("use code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	code = Run([]string{"configure", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "* prod\thttps://prod.example") {
		t.Fatalf("unexpected list output: %s", stdout.String())
	}
}

func TestConfigureDeleteCurrentProfileFallsBackToFirstRemainingProfile(t *testing.T) {
	t.Setenv("FLATRUN_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, args := range [][]string{
		{"configure", "set", "--profile", "zeta", "--url", "https://zeta.example", "--token", "zeta-token"},
		{"configure", "set", "--profile", "alpha", "--url", "https://alpha.example", "--token", "alpha-token"},
		{"configure", "delete", "alpha"},
	} {
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v code=%d stderr=%s", args, code, stderr.String())
		}
	}
	stdout.Reset()
	stderr.Reset()

	code := Run([]string{"configure", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "* zeta\thttps://zeta.example") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestConfigureDeleteLastCurrentProfileLeavesNoActiveProfile(t *testing.T) {
	t.Setenv("FLATRUN_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"configure", "set", "--url", "https://panel.example.com", "--token", "secret"}, &stdout, &stderr); code != 0 {
		t.Fatalf("set code=%d stderr=%s", code, stderr.String())
	}
	if code := Run([]string{"configure", "delete", "default"}, &stdout, &stderr); code != 0 {
		t.Fatalf("delete code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	code := Run([]string{"health"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no active FlatRun profile") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestInterspersedFlagsAllowsAWSStyleOrdering(t *testing.T) {
	args := interspersedFlags(
		[]string{"my-app", "--operation", "rebuild", "--json"},
		valueFlags("operation"),
	)

	want := []string{"--operation", "rebuild", "--json", "my-app"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestInterspersedFlagsDoesNotConsumeNextFlagAsMissingValue(t *testing.T) {
	args := interspersedFlags(
		[]string{"my-app", "--operation", "--json"},
		valueFlags("operation"),
	)

	want := []string{"--operation=", "--json", "my-app"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestInterspersedFlagsHandlesEqualsAndDashPositionals(t *testing.T) {
	args := interspersedFlags(
		[]string{"-", "--operation=rebuild", "my-app", "--json"},
		valueFlags("operation"),
	)

	want := []string{"--operation=rebuild", "--json", "-", "my-app"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestDeploymentDeployRejectsInvalidOperationAfterPositional(t *testing.T) {
	t.Setenv("FLATRUN_URL", "https://panel.example.com")
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "deploy", "my-app", "--operation", "delete"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--operation must be restart") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestOldColonCommandsAreNotAccepted(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"manage:deploy", "my-app"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Unknown command: manage:deploy") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestNormalizeAPIPath(t *testing.T) {
	tests := map[string]string{
		"settings":       "/settings",
		"/settings":      "/settings",
		"/api/settings":  "/settings",
		"/api":           "/",
		"/apiary/status": "/apiary/status",
		"apiary/status":  "/apiary/status",
	}

	for input, want := range tests {
		if got := normalizeAPIPath(input); got != want {
			t.Fatalf("normalizeAPIPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHelpShowsResourceCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"deployment", "image", "container", "api"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDeploymentSubcommandRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"deployment", "unknown"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Unknown deployment command: unknown") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestDeploymentCreateSendsOnlyPortsShape(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/deployments" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Fatalf("authorization = %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_, _ = w.Write([]byte(`{"message":"created"}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "create", "api", "--image", "ghcr.io/acme/api:main", "--port", "8080", "--host-port", "18080"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, ok := payload["host_port"]; ok {
		t.Fatalf("host_port should not be sent when ports[] is sent: %+v", payload)
	}
	if _, ok := payload["container_port"]; ok {
		t.Fatalf("container_port should not be sent when ports[] is sent: %+v", payload)
	}
	ports, ok := payload["ports"].([]any)
	if !ok || len(ports) != 1 {
		t.Fatalf("ports = %#v", payload["ports"])
	}
	port, ok := ports[0].(map[string]any)
	if !ok {
		t.Fatalf("port = %#v", ports[0])
	}
	if port["host"] != "18080" || port["container"] != float64(8080) {
		t.Fatalf("port = %#v", port)
	}
}

func TestDeploymentDeleteRequiresConfirmation(t *testing.T) {
	t.Setenv("FLATRUN_URL", "https://panel.example.com")
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "delete", "prod"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "refusing to delete") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestDeploymentDeleteCallsAPIWithConfirmation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/deployments/prod" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("delete_ssl") != "true" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"message":"deleted"}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "delete", "prod", "--confirm", "prod"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestAPIPostSendsData(t *testing.T) {
	var payload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/databases/list" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_, _ = w.Write([]byte(`{"databases":[]}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"api", "post", "/databases/list", "--data", `{"container":"mysql"}`}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if payload["container"] != "mysql" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestDeploymentCreateHelpExitsZero(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"deployment", "create", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage of deployment create") {
		t.Fatalf("expected usage output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestClientFromOptionsReportsCorruptConfigEvenWithExplicitCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("FLATRUN_CONFIG", path)
	if err := os.WriteFile(path, []byte(`{`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"health", "--url", "https://panel.example.com", "--token", "secret"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unexpected end of JSON input") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestVersionPrintsBuildMetadata(t *testing.T) {
	oldVersion := Version
	oldBuildTime := BuildTime
	oldGitCommit := GitCommit
	Version = "1.2.3"
	BuildTime = "2026-05-21T10:00:00Z"
	GitCommit = "abcdef"
	t.Cleanup(func() {
		Version = oldVersion
		BuildTime = oldBuildTime
		GitCommit = oldGitCommit
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"1.2.3", "build_time=2026-05-21T10:00:00Z", "git_commit=abcdef"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("version output missing %q: %s", want, stdout.String())
		}
	}
}

func TestPrintResponsePrintsTopLevelArrayWhenNoFallback(t *testing.T) {
	var stdout bytes.Buffer

	printResponse(&stdout, false, []byte(`[{"name":"api"}]`), "")

	if strings.TrimSpace(stdout.String()) != `[{"name":"api"}]` {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
