package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/flatrun/cli/internal/config"
	"github.com/flatrun/cli/internal/flatrun"
	"gopkg.in/yaml.v3"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var stdin io.Reader = os.Stdin

var stdinIsTerminal = func() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

var deploymentOperations = map[string]bool{
	"restart": true,
	"rebuild": true,
	"start":   true,
}

type globalOptions struct {
	Profile  string
	URL      string
	Token    string
	Timeout  time.Duration
	Insecure bool
	JSON     bool
	Verbose  bool
	debugOut io.Writer
}

type clientCommand struct {
	name        string
	usage       string
	successMsg  string
	positionals int
	valueFlags  []string
	flags       func(*flag.FlagSet)
	run         func(context.Context, *flatrun.Client, []string) ([]byte, error)
	render      func(io.Writer, []byte) error
}

type deploymentListItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Services  []struct {
		Name string `json:"name"`
	} `json:"services"`
	Metadata struct {
		Networking struct {
			Domain string `json:"domain"`
		} `json:"networking"`
		Domains []struct {
			Domain string `json:"domain"`
		} `json:"domains"`
	} `json:"metadata"`
}

type deploymentServiceItem struct {
	Name        string   `json:"name"`
	ContainerID string   `json:"container_id"`
	Image       string   `json:"image"`
	Status      string   `json:"status"`
	Health      string   `json:"health"`
	Ports       []string `json:"ports"`
}

type imageListItem struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Size       int64    `json:"size"`
	Created    string   `json:"created"`
	Containers int      `json:"containers"`
}

type containerListItem struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Image  string   `json:"image"`
	State  string   `json:"state"`
	Status string   `json:"status"`
	Ports  []string `json:"ports"`
}

type deploymentImageItem struct {
	Service  string `json:"service"`
	Image    string `json:"image"`
	IsLatest bool   `json:"is_latest"`
	IsBuild  bool   `json:"is_build"`
}

type deploymentContainerItem struct {
	ContainerID   string  `json:"container_id"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   int64   `json:"memory_usage"`
	MemoryPercent float64 `json:"memory_percent"`
	PIDs          int     `json:"pids"`
}

type deploymentInfo struct {
	Name     string                  `json:"name"`
	Status   string                  `json:"status"`
	Services []deploymentServiceItem `json:"services"`
	Metadata struct {
		Networking struct {
			Domain        string `json:"domain"`
			Expose        bool   `json:"expose"`
			ContainerPort int    `json:"container_port"`
			Protocol      string `json:"protocol"`
			ProxyType     string `json:"proxy_type"`
		} `json:"networking"`
		SSL struct {
			Enabled  bool `json:"enabled"`
			AutoCert bool `json:"auto_cert"`
		} `json:"ssl"`
		Healthcheck struct {
			Path     string `json:"path"`
			Interval string `json:"interval"`
		} `json:"healthcheck"`
		Domains []struct {
			ID            string `json:"id"`
			Service       string `json:"service"`
			ContainerPort int    `json:"container_port"`
			Domain        string `json:"domain"`
			SSL           struct {
				Enabled  bool `json:"enabled"`
				AutoCert bool `json:"auto_cert"`
			} `json:"ssl"`
		} `json:"domains"`
		Databases []struct {
			ID       string `json:"id"`
			Alias    string `json:"alias"`
			Type     string `json:"type"`
			Mode     string `json:"mode"`
			IsShared bool   `json:"is_shared"`
		} `json:"databases"`
		CredentialID string `json:"credential_id"`
	} `json:"metadata"`
}

type proxyStatusInfo struct {
	Exposed           bool     `json:"exposed"`
	Domain            string   `json:"domain"`
	Domains           []string `json:"domains"`
	VirtualHostExists bool     `json:"virtual_host_exists"`
	SSLEnabled        bool     `json:"ssl_enabled"`
	CertificateExists bool     `json:"certificate_exists"`
	Certificate       struct {
		Domain    string `json:"domain"`
		Issuer    string `json:"issuer"`
		NotAfter  string `json:"not_after"`
		DaysLeft  int    `json:"days_left"`
		Status    string `json:"status"`
		AutoRenew bool   `json:"auto_renew"`
	} `json:"certificate"`
}

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	case "version", "--version":
		_, _ = fmt.Fprintf(stdout, "%s\nbuild_time=%s\ngit_commit=%s\n", Version, BuildTime, GitCommit)
		return 0
	case "configure":
		return runConfigure(args[1:], stdout, stderr)
	case "health":
		return runHealth(args[1:], stdout, stderr)
	case "deployment":
		return runDeployment(args[1:], stdout, stderr)
	case "image":
		return runImage(args[1:], stdout, stderr)
	case "container":
		return runContainer(args[1:], stdout, stderr)
	case "api":
		return runAPI(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "FlatRun CLI")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  flatrun <command> [options]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  configure set       Save a local profile")
	_, _ = fmt.Fprintln(w, "  configure list      List local profiles")
	_, _ = fmt.Fprintln(w, "  health              Check FlatRun API health")
	_, _ = fmt.Fprintln(w, "  deployment          Manage deployments and their services/images/containers")
	_, _ = fmt.Fprintln(w, "  image               Manage Docker images")
	_, _ = fmt.Fprintln(w, "  container           Manage containers")
	_, _ = fmt.Fprintln(w, "  api                 Call any FlatRun API endpoint")
	_, _ = fmt.Fprintln(w, "  version             Print CLI version")
}

func globalFlagSet(name string, opts *globalOptions, output, debugOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&opts.Profile, "profile", "", "Profile name")
	fs.StringVar(&opts.URL, "url", "", "FlatRun API URL")
	fs.StringVar(&opts.Token, "token", "", "FlatRun API token")
	fs.DurationVar(&opts.Timeout, "timeout", 10*time.Minute, "Request timeout")
	fs.BoolVar(&opts.Insecure, "insecure-skip-verify", false, "Skip TLS certificate verification")
	fs.BoolVar(&opts.JSON, "json", false, "Print raw JSON response")
	fs.BoolVar(&opts.Verbose, "verbose", false, "Print request and response diagnostics")
	opts.debugOut = debugOut
	return fs
}

func clientFromOptions(opts globalOptions) (*flatrun.Client, error) {
	profile, _, err := config.Resolve(opts.Profile)
	if err != nil {
		if opts.URL != "" && opts.Token != "" && !isCredentialError(err) {
			return nil, err
		}
		if opts.URL == "" || opts.Token == "" {
			return nil, err
		}
	}
	if opts.URL != "" {
		profile.URL = opts.URL
	}
	if opts.Token != "" {
		profile.Token = opts.Token
	}
	if profile.URL == "" {
		return nil, errors.New("missing --url or FLATRUN_URL")
	}
	if profile.Token == "" {
		return nil, errors.New("missing --token or FLATRUN_TOKEN")
	}
	client := flatrun.New(profile.URL, profile.Token, opts.Timeout, opts.Insecure)
	if opts.Verbose {
		client.Debug = opts.debugOut
	}
	return client, nil
}

func isCredentialError(err error) bool {
	return strings.Contains(err.Error(), "missing FlatRun URL") || strings.Contains(err.Error(), "missing FlatRun token")
}

func parseFlagSet(fs *flag.FlagSet, args []string) (int, bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	return 0, true
}

func runClientCommand(cmd clientCommand, args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	fs := globalFlagSet(cmd.name, &opts, stderr, stderr)
	if cmd.flags != nil {
		cmd.flags(fs)
	}
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags(cmd.valueFlags...))); !ok {
		return code
	}
	if fs.NArg() != cmd.positionals {
		_, _ = fmt.Fprintln(stderr, cmd.usage)
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := cmd.run(context.Background(), client, fs.Args())
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	if opts.JSON {
		printResponse(stdout, true, data, "")
		return 0
	}
	if cmd.render != nil {
		if err := cmd.render(stdout, data); err != nil {
			_, _ = fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		return 0
	}
	printResponse(stdout, false, data, cmd.successMsg)
	return 0
}

func runConfigure(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun configure <set|list>")
		return 2
	}
	switch args[0] {
	case "set":
		return runConfigureSet(args[1:], stdout, stderr)
	case "list":
		return runConfigureList(args[1:], stdout, stderr)
	case "use":
		return runConfigureUse(args[1:], stdout, stderr)
	case "delete":
		return runConfigureDelete(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown configure command: %s\n", args[0])
		return 2
	}
}

func runConfigureSet(args []string, stdout, stderr io.Writer) int {
	profileName := "default"
	urlValue := ""
	token := ""
	tokenStdin := false

	fs := flag.NewFlagSet("configure set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&profileName, "profile", profileName, "Profile name")
	fs.StringVar(&urlValue, "url", "", "FlatRun API URL")
	fs.StringVar(&token, "token", "", "FlatRun API token")
	fs.BoolVar(&tokenStdin, "token-stdin", false, "Read FlatRun API token from stdin")
	if code, ok := parseFlagSet(fs, args); !ok {
		return code
	}
	if tokenStdin {
		if token != "" {
			_, _ = fmt.Fprintln(stderr, "Error: --token and --token-stdin are mutually exclusive")
			return 2
		}
		if stdinIsTerminal() {
			_, _ = fmt.Fprintln(stderr, "Error: --token-stdin requires piped input")
			return 2
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		token = strings.TrimSpace(string(data))
	}
	if urlValue == "" {
		_, _ = fmt.Fprintln(stderr, "Error: --url is required")
		return 2
	}
	if token == "" {
		_, _ = fmt.Fprintln(stderr, "Error: --token is required")
		return 2
	}

	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	cfg.CurrentProfile = profileName
	cfg.Profiles[profileName] = config.Profile{URL: urlValue, Token: token}
	if err := config.Save(path, cfg); err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Saved profile %q\n", profileName)
	return 0
}

func runConfigureList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("configure list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if code, ok := parseFlagSet(fs, args); !ok {
		return code
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		marker := " "
		if name == cfg.CurrentProfile {
			marker = "*"
		}
		_, _ = fmt.Fprintf(stdout, "%s %s\t%s\n", marker, name, cfg.Profiles[name].URL)
	}
	return 0
}

func runConfigureUse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("configure use", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if code, ok := parseFlagSet(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun configure use PROFILE")
		return 2
	}

	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	profileName := fs.Arg(0)
	if _, ok := cfg.Profiles[profileName]; !ok {
		_, _ = fmt.Fprintf(stderr, "Error: profile %q does not exist\n", profileName)
		return 1
	}
	cfg.CurrentProfile = profileName
	if err := config.Save(path, cfg); err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Using profile %q\n", profileName)
	return 0
}

func runConfigureDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("configure delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if code, ok := parseFlagSet(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun configure delete PROFILE")
		return 2
	}

	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	profileName := fs.Arg(0)
	if _, ok := cfg.Profiles[profileName]; !ok {
		_, _ = fmt.Fprintf(stderr, "Error: profile %q does not exist\n", profileName)
		return 1
	}
	delete(cfg.Profiles, profileName)
	if cfg.CurrentProfile == profileName {
		cfg.CurrentProfile = nextProfileName(cfg.Profiles)
	}
	if err := config.Save(path, cfg); err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Deleted profile %q\n", profileName)
	return 0
}

func nextProfileName(profiles map[string]config.Profile) string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func runHealth(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	fs := globalFlagSet("health", &opts, stderr, stderr)
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags())); !ok {
		return code
	}
	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.Health(context.Background())
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, opts.JSON, data, "FlatRun API is reachable")
	return 0
}

func runDeployment(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment <list|info|get|actions|action|exec|image|create|delete|start|stop|restart|rebuild|deploy|pull|images|containers|services>")
		return 2
	}

	switch args[0] {
	case "list":
		return runDeploymentList(args[1:], stdout, stderr)
	case "info", "get":
		return runDeploymentInfo(args[0], args[1:], stdout, stderr)
	case "actions":
		return runDeploymentActions(args[1:], stdout, stderr)
	case "action":
		return runDeploymentAction(args[1:], stdout, stderr)
	case "exec":
		return runDeploymentExec(args[1:], stdout, stderr)
	case "image":
		return runDeploymentImage(args[1:], stdout, stderr)
	case "create":
		return runDeploymentCreate(args[1:], stdout, stderr)
	case "delete":
		return runDeploymentDelete(args[1:], stdout, stderr)
	case "start", "stop", "restart", "rebuild":
		return runDeploymentSimple(args[0], args[1:], stdout, stderr)
	case "deploy":
		return runDeploymentDeploy(args[1:], stdout, stderr)
	case "pull":
		return runDeploymentPull(args[1:], stdout, stderr)
	case "images", "containers", "services":
		return runDeploymentRead(args[0], args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown deployment command: %s\n", args[0])
		return 2
	}
}

func runDeploymentList(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "deployment list",
		usage:       "Usage: flatrun deployment list",
		positionals: 0,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ListDeployments(ctx)
		},
		render: renderDeploymentList,
	}, args, stdout, stderr)
}

func runDeploymentInfo(command string, args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "deployment " + command,
		usage:       "Usage: flatrun deployment " + command + " NAME",
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.GetDeployment(ctx, args[0])
		},
		render: renderDeploymentGet,
	}, args, stdout, stderr)
}

func runDeploymentActions(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "deployment actions",
		usage:       "Usage: flatrun deployment actions NAME",
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.GetDeployment(ctx, args[0])
		},
		render: renderQuickActions,
	}, args, stdout, stderr)
}

func runDeploymentAction(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "deployment action",
		usage:       "Usage: flatrun deployment action NAME ACTION_ID",
		successMsg:  "Action executed",
		positionals: 2,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ExecuteQuickAction(ctx, args[0], args[1])
		},
		render: renderQuickActionResult,
	}, args, stdout, stderr)
}

func runDeploymentExec(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	service := ""

	head, command := splitOnDoubleDash(args)

	fs := globalFlagSet("deployment exec", &opts, stderr, stderr)
	fs.StringVar(&service, "service", "", "Service whose container runs the command")
	if code, ok := parseFlagSet(fs, interspersedFlags(head, globalValueFlags("service"))); !ok {
		return code
	}

	positionals := fs.Args()
	if len(positionals) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment exec NAME [--service SERVICE] -- COMMAND [ARGS...]")
		return 2
	}
	name := positionals[0]
	if len(command) == 0 {
		command = positionals[1:]
	}
	if len(command) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment exec NAME [--service SERVICE] -- COMMAND [ARGS...]")
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}

	data, err := client.GetDeployment(context.Background(), name)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	containerID, err := serviceContainerID(data, service)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}

	return execContainer(client, containerID, command, opts, stdout, stderr)
}

func runContainerExec(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}

	head, command := splitOnDoubleDash(args)

	fs := globalFlagSet("container exec", &opts, stderr, stderr)
	if code, ok := parseFlagSet(fs, interspersedFlags(head, globalValueFlags())); !ok {
		return code
	}

	positionals := fs.Args()
	if len(positionals) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun container exec CONTAINER_ID -- COMMAND [ARGS...]")
		return 2
	}
	containerID := positionals[0]
	if len(command) == 0 {
		command = positionals[1:]
	}
	if len(command) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun container exec CONTAINER_ID -- COMMAND [ARGS...]")
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}

	return execContainer(client, containerID, command, opts, stdout, stderr)
}

func execContainer(client *flatrun.Client, containerID string, command []string, opts globalOptions, stdout, stderr io.Writer) int {
	data, err := client.ContainerExec(context.Background(), containerID, flatrun.ExecRequest{
		Command: command[0],
		Args:    command[1:],
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	if opts.JSON {
		printResponse(stdout, true, data, "")
		return 0
	}
	if err := renderExecOutput(stdout, data); err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	return 0
}

func splitOnDoubleDash(args []string) (head, command []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func serviceContainerID(data []byte, service string) (string, error) {
	var response struct {
		Deployment struct {
			Services []struct {
				Name        string `json:"name"`
				ContainerID string `json:"container_id"`
			} `json:"services"`
		} `json:"deployment"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", err
	}
	services := response.Deployment.Services
	if service != "" {
		for _, svc := range services {
			if svc.Name == service {
				if svc.ContainerID == "" {
					return "", fmt.Errorf("service %q has no running container", service)
				}
				return svc.ContainerID, nil
			}
		}
		return "", fmt.Errorf("service %q not found in deployment", service)
	}
	for _, svc := range services {
		if svc.ContainerID != "" {
			return svc.ContainerID, nil
		}
	}
	return "", errors.New("no running container found in deployment; specify --service")
}

func runDeploymentImage(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment image <set>")
		return 2
	}
	switch args[0] {
	case "set":
		return runDeploymentImageSet(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown deployment image command: %s\n", args[0])
		return 2
	}
}

func runDeploymentImageSet(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	deploy := false
	operation := "rebuild"
	pull := true
	onlyLatest := false

	fs := globalFlagSet("deployment image set", &opts, stderr, stderr)
	fs.BoolVar(&deploy, "deploy", false, "Pull and run deployment operation after updating compose")
	fs.StringVar(&operation, "operation", operation, "Deployment operation when --deploy is set: restart, rebuild, or start")
	fs.BoolVar(&pull, "pull", pull, "Pull images before deployment operation when --deploy is set")
	fs.BoolVar(&onlyLatest, "only-latest", onlyLatest, "Only pull images tagged latest when --deploy is set")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("operation"))); !ok {
		return code
	}
	if fs.NArg() != 3 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment image set DEPLOYMENT SERVICE IMAGE [--deploy]")
		return 2
	}
	if deploy && !deploymentOperations[operation] {
		_, _ = fmt.Fprintln(stderr, "Error: --operation must be restart, rebuild, or start")
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}

	deploymentName := fs.Arg(0)
	serviceName := fs.Arg(1)
	imageName := fs.Arg(2)

	data, err := client.GetDeploymentCompose(context.Background(), deploymentName)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	content, err := composeContentFromResponse(data)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	updated, oldImage, err := setComposeServiceImage(content, serviceName, imageName)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}

	data, err = client.UpdateDeploymentCompose(context.Background(), deploymentName, updated)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	if opts.JSON && !deploy {
		printResponse(stdout, true, data, "")
		return 0
	}
	if !opts.JSON {
		_, _ = fmt.Fprintf(stdout, "Updated %s image for deployment %s: %s -> %s\n", serviceName, deploymentName, oldImage, imageName)
	}

	if deploy {
		data, err = client.Deploy(context.Background(), deploymentName, flatrun.DeployRequest{
			Action:     operation,
			Pull:       pull,
			OnlyLatest: onlyLatest,
		})
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		if opts.JSON {
			printResponse(stdout, true, data, "")
			return 0
		}
		printResponse(stdout, false, data, "Deployment completed")
	}
	return 0
}

func runDeploymentCreate(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	image := ""
	templateID := ""
	containerPort := 0
	hostPort := ""
	autoStart := false

	fs := globalFlagSet("deployment create", &opts, stderr, stderr)
	fs.StringVar(&image, "image", "", "Container image to deploy")
	fs.StringVar(&templateID, "template", "", "Template ID")
	fs.IntVar(&containerPort, "port", 0, "Container port")
	fs.StringVar(&hostPort, "host-port", "", "Host port")
	fs.BoolVar(&autoStart, "start", false, "Start after creation")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("image", "template", "port", "host-port"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment create NAME [--image IMAGE]")
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}

	req := flatrun.CreateDeploymentRequest{
		Name:          fs.Arg(0),
		Image:         image,
		TemplateID:    templateID,
		ContainerPort: containerPort,
		AutoStart:     autoStart,
	}
	if hostPort != "" {
		req.MapPorts = true
		req.HostPort = hostPort
	}
	if hostPort != "" && containerPort != 0 {
		req.Ports = []flatrun.PortConfig{{Container: containerPort, Host: hostPort}}
		req.MapPorts = false
		req.HostPort = ""
		req.ContainerPort = 0
	}

	data, err := client.CreateDeployment(context.Background(), req)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, opts.JSON, data, "Deployment created")
	return 0
}

func runDeploymentDelete(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	deleteSSL := true
	deleteDatabase := false
	deleteVhost := true
	yes := false
	confirm := ""

	fs := globalFlagSet("deployment delete", &opts, stderr, stderr)
	fs.BoolVar(&deleteSSL, "delete-ssl", deleteSSL, "Delete SSL certificates")
	fs.BoolVar(&deleteDatabase, "delete-database", deleteDatabase, "Delete shared database resources")
	fs.BoolVar(&deleteVhost, "delete-vhost", deleteVhost, "Delete virtual host/proxy config")
	fs.BoolVar(&yes, "yes", false, "Confirm deletion")
	fs.StringVar(&confirm, "confirm", "", "Confirm deletion by repeating the deployment name")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("confirm"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment delete NAME")
		return 2
	}
	if !yes && confirm != fs.Arg(0) {
		_, _ = fmt.Fprintf(stderr, "Error: refusing to delete %q without --yes or --confirm %s\n", fs.Arg(0), fs.Arg(0))
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.DeleteDeployment(context.Background(), fs.Arg(0), flatrun.DeleteDeploymentOptions{
		DeleteSSL:      deleteSSL,
		DeleteDatabase: deleteDatabase,
		DeleteVhost:    deleteVhost,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, opts.JSON, data, "Deployment deleted")
	return 0
}

func runDeploymentSimple(operation string, args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "deployment " + operation,
		usage:       fmt.Sprintf("Usage: flatrun deployment %s NAME", operation),
		successMsg:  "Deployment " + operation + " completed",
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.Manage(ctx, args[0], operation)
		},
	}, args, stdout, stderr)
}

func runDeploymentPull(args []string, stdout, stderr io.Writer) int {
	onlyLatest := false
	return runClientCommand(clientCommand{
		name:        "deployment pull",
		usage:       "Usage: flatrun deployment pull NAME",
		successMsg:  "Images pulled",
		positionals: 1,
		flags: func(fs *flag.FlagSet) {
			fs.BoolVar(&onlyLatest, "only-latest", onlyLatest, "Only pull images tagged latest")
		},
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.PullImages(ctx, args[0], onlyLatest)
		},
	}, args, stdout, stderr)
}

func runDeploymentRead(kind string, args []string, stdout, stderr io.Writer) int {
	render := renderDeploymentServices
	if kind == "images" {
		render = renderDeploymentImages
	}
	if kind == "containers" {
		render = renderDeploymentContainers
	}
	return runClientCommand(clientCommand{
		name:        "deployment " + kind,
		usage:       fmt.Sprintf("Usage: flatrun deployment %s NAME", kind),
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			switch kind {
			case "images":
				return client.DeploymentImages(ctx, args[0])
			case "containers":
				return client.DeploymentContainers(ctx, args[0])
			case "services":
				return client.DeploymentServices(ctx, args[0])
			default:
				return nil, fmt.Errorf("unsupported deployment read: %s", kind)
			}
		},
		render: render,
	}, args, stdout, stderr)
}

func runDeploymentDeploy(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	operation := "restart"
	pull := true
	onlyLatest := false

	fs := globalFlagSet("deployment deploy", &opts, stderr, stderr)
	fs.StringVar(&operation, "operation", operation, "Operation after pull: restart, rebuild, or start")
	fs.BoolVar(&pull, "pull", pull, "Pull images before running the operation")
	fs.BoolVar(&onlyLatest, "only-latest", onlyLatest, "Only pull images tagged latest")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("operation"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun deployment deploy NAME [--operation restart|rebuild|start]")
		return 2
	}
	if !deploymentOperations[operation] {
		_, _ = fmt.Fprintln(stderr, "Error: --operation must be restart, rebuild, or start")
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.Deploy(context.Background(), fs.Arg(0), flatrun.DeployRequest{
		Action:     operation,
		Pull:       pull,
		OnlyLatest: onlyLatest,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, opts.JSON, data, "Deployment completed")
	return 0
}

func runImage(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun image <list|pull|delete>")
		return 2
	}

	switch args[0] {
	case "list":
		return runImageList(args[1:], stdout, stderr)
	case "pull":
		return runImagePull(args[1:], stdout, stderr)
	case "delete":
		return runImageDelete(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown image command: %s\n", args[0])
		return 2
	}
}

func runImageList(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "image list",
		usage:       "Usage: flatrun image list",
		positionals: 0,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ListImages(ctx)
		},
		render: renderImageList,
	}, args, stdout, stderr)
}

func runImagePull(args []string, stdout, stderr io.Writer) int {
	credentialID := ""
	return runClientCommand(clientCommand{
		name:        "image pull",
		usage:       "Usage: flatrun image pull IMAGE",
		successMsg:  "Image pulled",
		positionals: 1,
		valueFlags:  []string{"credential-id"},
		flags: func(fs *flag.FlagSet) {
			fs.StringVar(&credentialID, "credential-id", "", "Registry credential ID")
		},
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.PullImage(ctx, args[0], credentialID)
		},
	}, args, stdout, stderr)
}

func runImageDelete(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "image delete",
		usage:       "Usage: flatrun image delete IMAGE_ID",
		successMsg:  "Image deleted",
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.RemoveImage(ctx, args[0])
		},
	}, args, stdout, stderr)
}

func runContainer(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun container <list|start|stop|restart|exec|delete>")
		return 2
	}

	switch args[0] {
	case "list":
		return runContainerList(args[1:], stdout, stderr)
	case "start", "stop", "restart":
		return runContainerSimple(args[0], args[1:], stdout, stderr)
	case "exec":
		return runContainerExec(args[1:], stdout, stderr)
	case "delete":
		return runContainerDelete(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown container command: %s\n", args[0])
		return 2
	}
}

func runContainerList(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "container list",
		usage:       "Usage: flatrun container list",
		positionals: 0,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ListContainers(ctx)
		},
		render: renderContainerList,
	}, args, stdout, stderr)
}

func runContainerSimple(operation string, args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "container " + operation,
		usage:       fmt.Sprintf("Usage: flatrun container %s CONTAINER_ID", operation),
		successMsg:  "Container " + operation + " completed",
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ContainerOperation(ctx, args[0], operation)
		},
	}, args, stdout, stderr)
}

func runContainerDelete(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "container delete",
		usage:       "Usage: flatrun container delete CONTAINER_ID",
		successMsg:  "Container deleted",
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.RemoveContainer(ctx, args[0])
		},
	}, args, stdout, stderr)
}

func runAPI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: flatrun api <get|post|put|delete> PATH [--data JSON]")
		return 2
	}

	switch args[0] {
	case "get":
		return runRawAPI(http.MethodGet, args[1:], stdout, stderr)
	case "post":
		return runRawAPI(http.MethodPost, args[1:], stdout, stderr)
	case "put":
		return runRawAPI(http.MethodPut, args[1:], stdout, stderr)
	case "delete":
		return runRawAPI(http.MethodDelete, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown api command: %s\n", args[0])
		return 2
	}
}

func runRawAPI(method string, args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{JSON: true}
	dataArg := ""
	fs := globalFlagSet("api "+strings.ToLower(method), &opts, stderr, stderr)
	fs.StringVar(&dataArg, "data", "", "JSON request body")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("data"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintf(stderr, "Usage: flatrun api %s /path [--data JSON]\n", strings.ToLower(method))
		return 2
	}

	var payload any
	if dataArg != "" {
		if err := json.Unmarshal([]byte(dataArg), &payload); err != nil {
			_, _ = fmt.Fprintln(stderr, "Error: invalid --data JSON:", err)
			return 2
		}
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.Raw(context.Background(), method, normalizeAPIPath(fs.Arg(0)), payload)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, true, data, "")
	return 0
}

func normalizeAPIPath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path == "/api" {
		return "/"
	}
	if strings.HasPrefix(path, "/api/") {
		return strings.TrimPrefix(path, "/api")
	}
	return path
}

func printResponse(stdout io.Writer, rawJSON bool, data []byte, fallback string) {
	if rawJSON {
		_, _ = fmt.Fprintln(stdout, string(data))
		return
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		if fallback == "" {
			_, _ = fmt.Fprintln(stdout, string(data))
			return
		}
		_, _ = fmt.Fprintln(stdout, fallback)
		return
	}
	if message, ok := response["message"].(string); ok && strings.TrimSpace(message) != "" {
		_, _ = fmt.Fprintln(stdout, message)
		return
	}
	if status, ok := response["status"].(string); ok && strings.TrimSpace(status) != "" && fallback != "" {
		_, _ = fmt.Fprintf(stdout, "%s: %s\n", fallback, status)
		return
	}
	_, _ = fmt.Fprintln(stdout, fallback)
}

func renderDeploymentList(stdout io.Writer, data []byte) error {
	var response struct {
		Deployments []deploymentListItem `json:"deployments"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	tableRows := make([][]string, 0, len(response.Deployments))
	for _, item := range response.Deployments {
		tableRows = append(tableRows, []string{item.Name, item.Status, deploymentDomains(item), fmt.Sprintf("%d", len(item.Services)), shortTime(item.CreatedAt)})
	}
	writeTable(stdout, []string{"NAME", "STATUS", "DOMAIN", "SERVICES", "CREATED"}, tableRows)
	return nil
}

func renderDeploymentGet(stdout io.Writer, data []byte) error {
	var response struct {
		Deployment  deploymentInfo  `json:"deployment"`
		ProxyStatus proxyStatusInfo `json:"proxy_status"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	renderDeploymentSummary(stdout, response.Deployment, response.ProxyStatus)

	tableRows := make([][]string, 0, len(response.Deployment.Services))
	for _, item := range response.Deployment.Services {
		tableRows = append(tableRows, []string{item.Name, item.ContainerID, item.Image, item.Status, item.Health, strings.Join(item.Ports, ",")})
	}
	_, _ = fmt.Fprintln(stdout)
	writeTable(stdout, []string{"SERVICE", "CONTAINER", "IMAGE", "STATUS", "HEALTH", "PORTS"}, tableRows)
	return nil
}

func renderQuickActions(stdout io.Writer, data []byte) error {
	var response struct {
		Deployment struct {
			Metadata struct {
				QuickActions []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Service     string `json:"service"`
					Command     string `json:"command"`
					Description string `json:"description"`
				} `json:"quick_actions"`
			} `json:"metadata"`
		} `json:"deployment"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	actions := response.Deployment.Metadata.QuickActions
	tableRows := make([][]string, 0, len(actions))
	for _, action := range actions {
		tableRows = append(tableRows, []string{action.ID, action.Name, action.Service, action.Command, action.Description})
	}
	writeTable(stdout, []string{"ID", "NAME", "SERVICE", "COMMAND", "DESCRIPTION"}, tableRows)
	return nil
}

func renderQuickActionResult(stdout io.Writer, data []byte) error {
	var response struct {
		Message  string `json:"message"`
		ActionID string `json:"action_id"`
		Output   string `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	if strings.TrimSpace(response.Output) != "" {
		_, _ = fmt.Fprintln(stdout, strings.TrimRight(response.Output, "\n"))
	}
	if response.Message != "" {
		_, _ = fmt.Fprintln(stdout, response.Message)
	}
	return nil
}

func renderExecOutput(stdout io.Writer, data []byte) error {
	var response struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	if strings.TrimSpace(response.Output) != "" {
		_, _ = fmt.Fprintln(stdout, strings.TrimRight(response.Output, "\n"))
	}
	return nil
}

func renderImageList(stdout io.Writer, data []byte) error {
	var response struct {
		Images []imageListItem `json:"images"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	tableRows := make([][]string, 0, len(response.Images))
	for _, item := range response.Images {
		tableRows = append(tableRows, []string{item.ID, strings.Join(item.Tags, ","), humanBytes(item.Size), fmt.Sprintf("%d", item.Containers), shortDockerTime(item.Created)})
	}
	writeTable(stdout, []string{"IMAGE ID", "TAGS", "SIZE", "CONTAINERS", "CREATED"}, tableRows)
	return nil
}

func renderContainerList(stdout io.Writer, data []byte) error {
	var response struct {
		Containers []containerListItem `json:"containers"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	tableRows := make([][]string, 0, len(response.Containers))
	for _, item := range response.Containers {
		tableRows = append(tableRows, []string{item.ID, item.Name, item.Image, item.State, item.Status, strings.Join(item.Ports, ",")})
	}
	writeTable(stdout, []string{"CONTAINER ID", "NAME", "IMAGE", "STATE", "STATUS", "PORTS"}, tableRows)
	return nil
}

func renderDeploymentImages(stdout io.Writer, data []byte) error {
	var response struct {
		Images []deploymentImageItem `json:"images"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	tableRows := make([][]string, 0, len(response.Images))
	for _, item := range response.Images {
		tableRows = append(tableRows, []string{item.Service, item.Image, boolText(item.IsLatest), boolText(item.IsBuild)})
	}
	writeTable(stdout, []string{"SERVICE", "IMAGE", "LATEST", "BUILD"}, tableRows)
	return nil
}

func renderDeploymentContainers(stdout io.Writer, data []byte) error {
	var response struct {
		Services []deploymentContainerItem `json:"services"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	tableRows := make([][]string, 0, len(response.Services))
	for _, item := range response.Services {
		tableRows = append(tableRows, []string{item.ContainerID, item.Name, fmt.Sprintf("%.2f%%", item.CPUPercent), humanBytes(item.MemoryUsage), fmt.Sprintf("%.2f%%", item.MemoryPercent), fmt.Sprintf("%d", item.PIDs)})
	}
	writeTable(stdout, []string{"CONTAINER ID", "NAME", "CPU", "MEMORY", "MEMORY %", "PIDS"}, tableRows)
	return nil
}

func renderDeploymentServices(stdout io.Writer, data []byte) error {
	var response struct {
		Services []map[string]any `json:"services"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return err
	}
	tableRows := make([][]string, 0, len(response.Services))
	for _, service := range response.Services {
		tableRows = append(tableRows, []string{
			stringValue(service["name"]),
			stringValue(service["image"]),
			stringValue(service["status"]),
			stringValue(service["health"]),
		})
	}
	writeTable(stdout, []string{"SERVICE", "IMAGE", "STATUS", "HEALTH"}, tableRows)
	return nil
}

func renderDeploymentSummary(stdout io.Writer, deployment deploymentInfo, proxy proxyStatusInfo) {
	if deployment.Name == "" {
		return
	}
	writeKeyValues(stdout, [][]string{
		{"Name", deployment.Name},
		{"Status", deployment.Status},
		{"Exposed", boolText(deployment.Metadata.Networking.Expose || proxy.Exposed)},
		{"Domains", infoDomains(deployment, proxy)},
		{"Container Port", intText(deployment.Metadata.Networking.ContainerPort)},
		{"Protocol", deployment.Metadata.Networking.Protocol},
		{"Proxy Type", deployment.Metadata.Networking.ProxyType},
		{"Virtual Host", boolText(proxy.VirtualHostExists)},
		{"SSL", sslSummary(deployment, proxy)},
		{"Certificate", certificateSummary(proxy)},
		{"Healthcheck", healthcheckSummary(deployment)},
		{"Credential", deployment.Metadata.CredentialID},
		{"Databases", databaseSummary(deployment)},
	})
}

func composeContentFromResponse(data []byte) (string, error) {
	var response struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Content) == "" {
		return "", errors.New("deployment compose response did not include content")
	}
	return response.Content, nil
}

func setComposeServiceImage(content, serviceName, imageName string) (string, string, error) {
	if strings.TrimSpace(serviceName) == "" {
		return "", "", errors.New("service name is required")
	}
	if strings.TrimSpace(imageName) == "" {
		return "", "", errors.New("image is required")
	}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return "", "", err
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return "", "", errors.New("compose content must be a YAML mapping")
	}
	services := mappingValue(root.Content[0], "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return "", "", errors.New("compose content does not contain services")
	}
	service := mappingValue(services, serviceName)
	if service == nil || service.Kind != yaml.MappingNode {
		return "", "", fmt.Errorf("service %q not found in compose", serviceName)
	}
	image := mappingValue(service, "image")
	if image == nil {
		return "", "", fmt.Errorf("service %q does not define an image", serviceName)
	}
	if image.Kind != yaml.ScalarNode {
		return "", "", fmt.Errorf("service %q image is not a scalar value", serviceName)
	}

	oldImage := image.Value
	image.Value = imageName
	image.Tag = "!!str"

	var output strings.Builder
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(root.Content[0]); err != nil {
		_ = encoder.Close()
		return "", "", err
	}
	if err := encoder.Close(); err != nil {
		return "", "", err
	}
	return output.String(), oldImage, nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func writeKeyValues(stdout io.Writer, pairs [][]string) {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for _, pair := range pairs {
		if len(pair) != 2 || pair[1] == "" {
			continue
		}
		_, _ = fmt.Fprintf(tw, "%s:\t%s\n", pair[0], pair[1])
	}
	_ = tw.Flush()
}

func infoDomains(deployment deploymentInfo, proxy proxyStatusInfo) string {
	domains := make([]string, 0, len(deployment.Metadata.Domains)+len(proxy.Domains)+1)
	seen := map[string]bool{}
	add := func(domain string) {
		if domain == "" || seen[domain] {
			return
		}
		seen[domain] = true
		domains = append(domains, domain)
	}
	for _, domain := range deployment.Metadata.Domains {
		add(domain.Domain)
	}
	for _, domain := range proxy.Domains {
		add(domain)
	}
	add(proxy.Domain)
	add(deployment.Metadata.Networking.Domain)
	return strings.Join(domains, ",")
}

func sslSummary(deployment deploymentInfo, proxy proxyStatusInfo) string {
	enabled := deployment.Metadata.SSL.Enabled || proxy.SSLEnabled
	if !enabled {
		return "disabled"
	}
	parts := []string{"enabled"}
	if deployment.Metadata.SSL.AutoCert {
		parts = append(parts, "auto-cert")
	}
	if proxy.CertificateExists {
		parts = append(parts, "certificate present")
	}
	return strings.Join(parts, ", ")
}

func certificateSummary(proxy proxyStatusInfo) string {
	cert := proxy.Certificate
	if cert.Domain == "" && !proxy.CertificateExists {
		return ""
	}
	parts := []string{}
	if cert.Domain != "" {
		parts = append(parts, cert.Domain)
	}
	if cert.Status != "" {
		parts = append(parts, cert.Status)
	}
	if cert.DaysLeft != 0 {
		parts = append(parts, fmt.Sprintf("%d days left", cert.DaysLeft))
	}
	if cert.Issuer != "" {
		parts = append(parts, "issuer "+cert.Issuer)
	}
	if cert.AutoRenew {
		parts = append(parts, "auto-renew")
	}
	return strings.Join(parts, ", ")
}

func healthcheckSummary(deployment deploymentInfo) string {
	if deployment.Metadata.Healthcheck.Path == "" && deployment.Metadata.Healthcheck.Interval == "" {
		return ""
	}
	if deployment.Metadata.Healthcheck.Interval == "" {
		return deployment.Metadata.Healthcheck.Path
	}
	if deployment.Metadata.Healthcheck.Path == "" {
		return deployment.Metadata.Healthcheck.Interval
	}
	return deployment.Metadata.Healthcheck.Path + " every " + deployment.Metadata.Healthcheck.Interval
}

func databaseSummary(deployment deploymentInfo) string {
	items := make([]string, 0, len(deployment.Metadata.Databases))
	for _, database := range deployment.Metadata.Databases {
		name := database.Alias
		if name == "" {
			name = database.ID
		}
		if database.Type != "" {
			name += " (" + database.Type
			if database.Mode != "" {
				name += ", " + database.Mode
			}
			if database.IsShared && database.Mode != "shared" {
				name += ", shared"
			}
			name += ")"
		}
		items = append(items, name)
	}
	return strings.Join(items, ",")
}

func writeTable(stdout io.Writer, headers []string, tableRows [][]string) {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range tableRows {
		_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	_ = tw.Flush()
}

func boolText(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func intText(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func deploymentDomains(item deploymentListItem) string {
	domains := make([]string, 0, len(item.Metadata.Domains))
	seen := map[string]bool{}
	for _, domain := range item.Metadata.Domains {
		if domain.Domain == "" || seen[domain.Domain] {
			continue
		}
		seen[domain.Domain] = true
		domains = append(domains, domain.Domain)
	}
	if len(domains) > 0 {
		return strings.Join(domains, ",")
	}
	return item.Metadata.Networking.Domain
}

func humanBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}

func shortTime(value string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.Format("2006-01-02 15:04")
	}
	return value
}

func shortDockerTime(value string) string {
	if parsed, err := time.Parse("2006-01-02 15:04:05 -0700 MST", value); err == nil {
		return parsed.Format("2006-01-02 15:04:05")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.Format("2006-01-02 15:04")
	}
	return value
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func valueFlags(names ...string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		result["--"+name] = true
	}
	return result
}

func globalValueFlags(extra ...string) map[string]bool {
	return valueFlags(append([]string{"profile", "url", "token", "timeout"}, extra...)...)
}

func interspersedFlags(args []string, flagsWithValues map[string]bool) []string {
	var flags []string
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}

		flags = append(flags, arg)
		name := arg
		if idx := strings.Index(arg, "="); idx >= 0 {
			name = arg[:idx]
		}
		if flagsWithValues[name] && !strings.Contains(arg, "=") {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") && args[i+1] != "-" {
				flags[len(flags)-1] = arg + "="
				continue
			}
			i++
			flags = append(flags, args[i])
		}
	}

	return append(flags, positionals...)
}
