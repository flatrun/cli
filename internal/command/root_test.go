package command

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDeploymentFilesPushSendsOneArchiveRequest(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "index.html"), []byte("home"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "assets", "app.js"), []byte("app"), 0o644); err != nil {
		t.Fatal(err)
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/api/deployments/whilesmart/files-push" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		fields := map[string]string{}
		files := map[string]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Error(err)
				return
			}
			if part.FormName() != "archive" {
				value, _ := io.ReadAll(part)
				fields[part.FormName()] = string(value)
				continue
			}
			gzipReader, err := gzip.NewReader(part)
			if err != nil {
				t.Error(err)
				return
			}
			tarReader := tar.NewReader(gzipReader)
			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Error(err)
					return
				}
				if header.Typeflag == tar.TypeReg {
					content, _ := io.ReadAll(tarReader)
					files[header.Name] = string(content)
				}
			}
		}
		if fields["destination"] != "sites/" || fields["delete"] != "true" {
			t.Errorf("fields = %#v", fields)
		}
		if files["index.html"] != "home" || files["assets/app.js"] != "app" {
			t.Errorf("files = %#v", files)
		}
		_, _ = w.Write([]byte(`{"item":{"message":"Pushed 2 files","destination":"sites/","deleted":true,"files":2}}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"deployment", "files", "push", "whilesmart", source, "sites/", "--delete"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
}

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

func TestDeploymentActionExecutesAndPrintsOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/deployments/api/actions/migrate" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"message":"Action executed successfully","action_id":"migrate","output":"Migrating: 2024_01_01_create\nMigrated:  2024_01_01_create"}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "action", "api", "migrate"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"Migrated:  2024_01_01_create", "Action executed successfully"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %s", want, stdout.String())
		}
	}
}

func TestDeploymentActionRequiresActionID(t *testing.T) {
	t.Setenv("FLATRUN_URL", "https://panel.example.com")
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "action", "api"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestDeploymentActionsListsQuickActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/deployments/api" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"deployment":{"name":"api","metadata":{"quick_actions":[{"id":"migrate","name":"Run migrations","service":"app","command":"php artisan migrate --force","description":"Apply pending migrations"}]}}}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "actions", "api"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"ID", "migrate", "Run migrations", "app", "php artisan migrate --force"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
	}
}

func TestContainerExecRunsCommandAndPrintsOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/containers/abc123/exec" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Command != "php" || strings.Join(body.Args, " ") != "artisan migrate --force" {
			t.Fatalf("body = %+v", body)
		}
		_, _ = w.Write([]byte(`{"output":"Nothing to migrate."}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"container", "exec", "abc123", "--", "php", "artisan", "migrate", "--force"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Nothing to migrate.") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestContainerExecPrintsOutputWhenCommandFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"exit status 127","output":"sh: php: not found"}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"container", "exec", "abc123", "--", "php", "-v"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr.String(), "sh: php: not found") {
		t.Fatalf("stderr missing command output: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "exit status 127") {
		t.Fatalf("stderr missing error: %s", stderr.String())
	}
}

func TestDeploymentExecResolvesServiceContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/api":
			_, _ = w.Write([]byte(`{"deployment":{"name":"api","services":[{"name":"app","container_id":"ctr-app"},{"name":"worker","container_id":"ctr-worker"}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/containers/ctr-worker/exec":
			_, _ = w.Write([]byte(`{"output":"OPTIMIZED"}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "exec", "api", "--service", "worker", "--", "php", "artisan", "optimize"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OPTIMIZED") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDeploymentExecAcceptsServiceAsPositional(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/api":
			_, _ = w.Write([]byte(`{"deployment":{"name":"api","services":[{"name":"app","container_id":"ctr-app"},{"name":"worker","container_id":"ctr-worker"}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/containers/ctr-app/exec":
			_, _ = w.Write([]byte(`{"output":"OK"}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "exec", "api", "app", "--", "php", "artisan", "migrate"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDeploymentExecErrorsWhenServiceAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/deployments/api" {
			_, _ = w.Write([]byte(`{"deployment":{"name":"api","services":[{"name":"app","container_id":"ctr-app"},{"name":"worker","container_id":"ctr-worker"}]}}`))
			return
		}
		t.Fatalf("unexpected request to %s; exec should not run when the service is ambiguous", r.URL.Path)
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "exec", "api", "--", "php", "-v"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"multiple services", "app", "worker"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q: %s", want, stderr.String())
		}
	}
}

func TestDeploymentExecRequiresDoubleDash(t *testing.T) {
	t.Setenv("FLATRUN_URL", "https://panel.example.com")
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// No `--` separator: the command is not clearly delimited, so this is rejected.
	code := Run([]string{"deployment", "exec", "api", "php", "artisan", "migrate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestDeploymentExecRequiresCommand(t *testing.T) {
	t.Setenv("FLATRUN_URL", "https://panel.example.com")
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "exec", "api"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestDeploymentListPrintsTableByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/deployments" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"deployments":[{"name":"api","status":"running","created_at":"2026-05-23T01:02:03Z","services":[{"name":"app"}],"metadata":{"networking":{"domain":"api.example.com"},"domains":[{"domain":"api.example.com"},{"domain":"admin.example.com"}]}}]}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"NAME", "api", "running", "api.example.com,admin.example.com", "2026-05-23 01:02"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), `"name"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDeploymentListPrintsJSONWhenRequested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"deployments":[{"name":"api"}]}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "list", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name":"api"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDeploymentInfoPrintsDeploymentDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/deployments/api" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"deployment":{"name":"api","status":"running","services":[{"name":"app","container_id":"abc123","image":"ghcr.io/acme/api:latest","status":"running","health":"healthy","ports":["80"]}],"metadata":{"networking":{"expose":true,"domain":"api.example.com","container_port":80,"protocol":"http","proxy_type":"http"},"ssl":{"enabled":true,"auto_cert":true},"healthcheck":{"path":"/health","interval":"30s"},"domains":[{"domain":"api.example.com"},{"domain":"admin.example.com"}],"databases":[{"id":"primary","alias":"primary","type":"mysql","mode":"shared","is_shared":true}],"credential_id":"cred123"}},"proxy_status":{"exposed":true,"domain":"api.example.com","domains":["api.example.com","admin.example.com"],"virtual_host_exists":true,"ssl_enabled":true,"certificate_exists":true,"certificate":{"domain":"api.example.com","issuer":"E8","days_left":34,"status":"valid","auto_renew":true}}}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "info", "api"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"Name:", "api", "Domains:", "api.example.com,admin.example.com", "SSL:", "enabled, auto-cert, certificate present", "Certificate:", "valid", "34 days left", "Healthcheck:", "/health every 30s", "Databases:", "primary (mysql, shared)", "SERVICE", "abc123", "healthy"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %q: %s", want, stdout.String())
		}
	}
}

func TestDeploymentGetStillWorksAsAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/deployments/api" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"deployment":{"name":"api","status":"running","services":[]}}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "get", "api"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Name:") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestDeploymentImageSetUpdatesComposeServiceImage(t *testing.T) {
	var updatedPayload map[string]string
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/api/compose":
			_, _ = w.Write([]byte(`{"content":"name: api\nservices:\n  app:\n    image: ghcr.io/acme/api:old\n  worker:\n    image: ghcr.io/acme/worker:old\n"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/deployments/api":
			if err := json.NewDecoder(r.Body).Decode(&updatedPayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"message":"Deployment updated"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "image", "set", "api", "app", "ghcr.io/acme/api:new"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Join(requests, "\n") != "GET /api/deployments/api/compose\nPUT /api/deployments/api" {
		t.Fatalf("requests = %#v", requests)
	}
	compose := updatedPayload["compose_content"]
	if !strings.Contains(compose, "image: ghcr.io/acme/api:new") {
		t.Fatalf("compose missing updated image:\n%s", compose)
	}
	if !strings.Contains(compose, "image: ghcr.io/acme/worker:old") {
		t.Fatalf("compose should preserve worker image:\n%s", compose)
	}
	if !strings.Contains(stdout.String(), "ghcr.io/acme/api:old -> ghcr.io/acme/api:new") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestDeploymentImageSetCanDeployAfterUpdate(t *testing.T) {
	var deployPayload map[string]any
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/api/compose":
			_, _ = w.Write([]byte(`{"content":"services:\n  app:\n    image: old\n"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/deployments/api":
			_, _ = w.Write([]byte(`{"message":"Deployment updated"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/api/deploy":
			if err := json.NewDecoder(r.Body).Decode(&deployPayload); err != nil {
				t.Fatalf("decode deploy payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"message":"Deployment completed"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"deployment", "image", "set", "api", "app", "new", "--deploy", "--operation", "restart"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Join(requests, "\n") != "GET /api/deployments/api/compose\nPUT /api/deployments/api\nPOST /api/deployments/api/deploy" {
		t.Fatalf("requests = %#v", requests)
	}
	if deployPayload["action"] != "restart" || deployPayload["pull"] != true {
		t.Fatalf("deploy payload = %+v", deployPayload)
	}
}

func TestSetComposeServiceImageRejectsMissingService(t *testing.T) {
	_, _, err := setComposeServiceImage("services:\n  app:\n    image: old\n", "worker", "new")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `service "worker" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImageListPrintsTableByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/images" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"images":[{"id":"abc123","tags":["ghcr.io/acme/api:latest"],"size":10485760,"created":"2026-05-23 01:02:03 +0100 CET","containers":2}]}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"image", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"IMAGE ID", "abc123", "ghcr.io/acme/api:latest", "10.0 MB", "2"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), `"images"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestContainerListPrintsTableByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/containers" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"containers":[{"id":"abc123","name":"api","image":"ghcr.io/acme/api:latest","state":"running","status":"Up 1 hour","ports":["0.0.0.0:80->80/tcp"]}]}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"container", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"CONTAINER ID", "abc123", "api", "running", "0.0.0.0:80->80/tcp"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
	}
}

func TestRenderDeploymentGetPrintsServiceTable(t *testing.T) {
	var stdout bytes.Buffer

	err := renderDeploymentGet(&stdout, []byte(`{"deployment":{"name":"api","status":"running","services":[{"name":"app","container_id":"abc123","image":"ghcr.io/acme/api:latest","status":"running","health":"healthy","ports":["80"]}]}}`))
	if err != nil {
		t.Fatalf("renderDeploymentGet returned error: %v", err)
	}
	for _, want := range []string{"Name:", "api", "SERVICE", "CONTAINER", "abc123", "healthy", "80"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
	}
}

func TestRenderDeploymentImagesPrintsImageTable(t *testing.T) {
	var stdout bytes.Buffer

	err := renderDeploymentImages(&stdout, []byte(`{"images":[{"service":"app","image":"ghcr.io/acme/api:latest","is_latest":true,"is_build":false}]}`))
	if err != nil {
		t.Fatalf("renderDeploymentImages returned error: %v", err)
	}
	for _, want := range []string{"SERVICE", "app", "ghcr.io/acme/api:latest", "yes", "no"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
	}
}

func TestRenderDeploymentContainersPrintsStatsTable(t *testing.T) {
	var stdout bytes.Buffer

	err := renderDeploymentContainers(&stdout, []byte(`{"services":[{"container_id":"abc123","name":"api","cpu_percent":1.25,"memory_usage":1048576,"memory_percent":2.5,"pids":7}]}`))
	if err != nil {
		t.Fatalf("renderDeploymentContainers returned error: %v", err)
	}
	for _, want := range []string{"CONTAINER ID", "abc123", "api", "1.25%", "1.0 MB", "2.50%", "7"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
	}
}

func TestRenderDeploymentServicesPrintsFlexibleServiceTable(t *testing.T) {
	var stdout bytes.Buffer

	err := renderDeploymentServices(&stdout, []byte(`{"services":[{"name":"app","image":"ghcr.io/acme/api:latest","status":"running","health":"healthy"}]}`))
	if err != nil {
		t.Fatalf("renderDeploymentServices returned error: %v", err)
	}
	for _, want := range []string{"SERVICE", "app", "ghcr.io/acme/api:latest", "running", "healthy"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("table missing %q: %s", want, stdout.String())
		}
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

func TestVerbosePrintsRequestDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer server.Close()

	t.Setenv("FLATRUN_URL", server.URL)
	t.Setenv("FLATRUN_TOKEN", "secret")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"health", "--verbose"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"-> GET " + server.URL + "/api/health", "<- 200 200 OK"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("verbose output missing %q: %s", want, stderr.String())
		}
	}
	if !regexp.MustCompile(`(?m)^<- body [0-9]+ bytes$`).MatchString(stderr.String()) {
		t.Fatalf("verbose output missing body size: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "secret") {
		t.Fatalf("verbose output leaked token: %s", stderr.String())
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

func TestUpdateRejectsDevelopmentBuild(t *testing.T) {
	oldVersion := Version
	Version = "dev"
	t.Cleanup(func() { Version = oldVersion })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"update"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "development builds cannot be updated automatically") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestUpdateRejectsArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"update", "unexpected"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: flatrun update [--check]") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestPrintResponsePrintsTopLevelArrayWhenNoFallback(t *testing.T) {
	var stdout bytes.Buffer

	printResponse(&stdout, false, []byte(`[{"name":"api"}]`), "")

	if strings.TrimSpace(stdout.String()) != `[{"name":"api"}]` {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestHumanBytesFormatsBoundaries(t *testing.T) {
	tests := map[int64]string{
		512:                  "512 B",
		1024:                 "1.0 KB",
		1048576:              "1.0 MB",
		1073741824:           "1.0 GB",
		1099511627776:        "1.0 TB",
		1125899906842624 * 2: "2.0 PB",
	}

	for input, want := range tests {
		if got := humanBytes(input); got != want {
			t.Fatalf("humanBytes(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestDeploymentDomainsFallsBackToLegacyDomain(t *testing.T) {
	item := deploymentListItem{}
	item.Metadata.Networking.Domain = "api.example.com"

	if got := deploymentDomains(item); got != "api.example.com" {
		t.Fatalf("deploymentDomains = %q", got)
	}
}

func TestDeploymentDomainsDeduplicatesConfiguredDomains(t *testing.T) {
	item := deploymentListItem{}
	item.Metadata.Networking.Domain = "legacy.example.com"
	item.Metadata.Domains = []struct {
		Domain string `json:"domain"`
	}{
		{Domain: "api.example.com"},
		{Domain: "api.example.com"},
		{Domain: "admin.example.com"},
	}

	if got := deploymentDomains(item); got != "api.example.com,admin.example.com" {
		t.Fatalf("deploymentDomains = %q", got)
	}
}

func TestShortDockerTimeParsesDockerAndRFC3339Formats(t *testing.T) {
	tests := map[string]string{
		"2026-05-23 01:02:03 +0100 CET": "2026-05-23 01:02:03",
		"2026-05-23T01:02:03Z":          "2026-05-23 01:02",
		"not-a-time":                    "not-a-time",
	}

	for input, want := range tests {
		if got := shortDockerTime(input); got != want {
			t.Fatalf("shortDockerTime(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
