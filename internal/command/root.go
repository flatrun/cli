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
	"time"

	"github.com/flatrun/cli/internal/config"
	"github.com/flatrun/cli/internal/flatrun"
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
}

type clientCommand struct {
	name        string
	usage       string
	rawJSON     bool
	successMsg  string
	positionals int
	valueFlags  []string
	flags       func(*flag.FlagSet)
	run         func(context.Context, *flatrun.Client, []string) ([]byte, error)
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
		fmt.Fprintf(stdout, "%s\nbuild_time=%s\ngit_commit=%s\n", Version, BuildTime, GitCommit)
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
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "FlatRun CLI")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  flatrun <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  configure set       Save a local profile")
	fmt.Fprintln(w, "  configure list      List local profiles")
	fmt.Fprintln(w, "  health              Check FlatRun API health")
	fmt.Fprintln(w, "  deployment          Manage deployments and their services/images/containers")
	fmt.Fprintln(w, "  image               Manage Docker images")
	fmt.Fprintln(w, "  container           Manage containers")
	fmt.Fprintln(w, "  api                 Call any FlatRun API endpoint")
	fmt.Fprintln(w, "  version             Print CLI version")
}

func globalFlagSet(name string, opts *globalOptions, output io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&opts.Profile, "profile", "", "Profile name")
	fs.StringVar(&opts.URL, "url", "", "FlatRun API URL")
	fs.StringVar(&opts.Token, "token", "", "FlatRun API token")
	fs.DurationVar(&opts.Timeout, "timeout", 10*time.Minute, "Request timeout")
	fs.BoolVar(&opts.Insecure, "insecure-skip-verify", false, "Skip TLS certificate verification")
	fs.BoolVar(&opts.JSON, "json", false, "Print raw JSON response")
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
	return flatrun.New(profile.URL, profile.Token, opts.Timeout, opts.Insecure), nil
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
	opts := globalOptions{JSON: cmd.rawJSON}
	fs := globalFlagSet(cmd.name, &opts, stderr)
	if cmd.flags != nil {
		cmd.flags(fs)
	}
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags(cmd.valueFlags...))); !ok {
		return code
	}
	if fs.NArg() != cmd.positionals {
		fmt.Fprintln(stderr, cmd.usage)
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := cmd.run(context.Background(), client, fs.Args())
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, opts.JSON, data, cmd.successMsg)
	return 0
}

func runConfigure(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: flatrun configure <set|list>")
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
		fmt.Fprintf(stderr, "Unknown configure command: %s\n", args[0])
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
			fmt.Fprintln(stderr, "Error: --token and --token-stdin are mutually exclusive")
			return 2
		}
		if stdinIsTerminal() {
			fmt.Fprintln(stderr, "Error: --token-stdin requires piped input")
			return 2
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		token = strings.TrimSpace(string(data))
	}
	if urlValue == "" {
		fmt.Fprintln(stderr, "Error: --url is required")
		return 2
	}
	if token == "" {
		fmt.Fprintln(stderr, "Error: --token is required")
		return 2
	}

	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	cfg.CurrentProfile = profileName
	cfg.Profiles[profileName] = config.Profile{URL: urlValue, Token: token}
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Saved profile %q\n", profileName)
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
		fmt.Fprintln(stderr, "Error:", err)
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
		fmt.Fprintf(stdout, "%s %s\t%s\n", marker, name, cfg.Profiles[name].URL)
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
		fmt.Fprintln(stderr, "Usage: flatrun configure use PROFILE")
		return 2
	}

	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	profileName := fs.Arg(0)
	if _, ok := cfg.Profiles[profileName]; !ok {
		fmt.Fprintf(stderr, "Error: profile %q does not exist\n", profileName)
		return 1
	}
	cfg.CurrentProfile = profileName
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Using profile %q\n", profileName)
	return 0
}

func runConfigureDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("configure delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if code, ok := parseFlagSet(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: flatrun configure delete PROFILE")
		return 2
	}

	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	profileName := fs.Arg(0)
	if _, ok := cfg.Profiles[profileName]; !ok {
		fmt.Fprintf(stderr, "Error: profile %q does not exist\n", profileName)
		return 1
	}
	delete(cfg.Profiles, profileName)
	if cfg.CurrentProfile == profileName {
		cfg.CurrentProfile = nextProfileName(cfg.Profiles)
	}
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Deleted profile %q\n", profileName)
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
	fs := globalFlagSet("health", &opts, stderr)
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags())); !ok {
		return code
	}
	client, err := clientFromOptions(opts)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.Health(context.Background())
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, opts.JSON, data, "FlatRun API is reachable")
	return 0
}

func runDeployment(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: flatrun deployment <list|get|create|delete|start|stop|restart|rebuild|deploy|pull|images|containers|services>")
		return 2
	}

	switch args[0] {
	case "list":
		return runDeploymentList(args[1:], stdout, stderr)
	case "get":
		return runDeploymentGet(args[1:], stdout, stderr)
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
		fmt.Fprintf(stderr, "Unknown deployment command: %s\n", args[0])
		return 2
	}
}

func runDeploymentList(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "deployment list",
		usage:       "Usage: flatrun deployment list",
		rawJSON:     true,
		positionals: 0,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ListDeployments(ctx)
		},
	}, args, stdout, stderr)
}

func runDeploymentGet(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "deployment get",
		usage:       "Usage: flatrun deployment get NAME",
		rawJSON:     true,
		positionals: 1,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.GetDeployment(ctx, args[0])
		},
	}, args, stdout, stderr)
}

func runDeploymentCreate(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	image := ""
	templateID := ""
	containerPort := 0
	hostPort := ""
	autoStart := false

	fs := globalFlagSet("deployment create", &opts, stderr)
	fs.StringVar(&image, "image", "", "Container image to deploy")
	fs.StringVar(&templateID, "template", "", "Template ID")
	fs.IntVar(&containerPort, "port", 0, "Container port")
	fs.StringVar(&hostPort, "host-port", "", "Host port")
	fs.BoolVar(&autoStart, "start", false, "Start after creation")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("image", "template", "port", "host-port"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: flatrun deployment create NAME [--image IMAGE]")
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
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
		fmt.Fprintln(stderr, "Error:", err)
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

	fs := globalFlagSet("deployment delete", &opts, stderr)
	fs.BoolVar(&deleteSSL, "delete-ssl", deleteSSL, "Delete SSL certificates")
	fs.BoolVar(&deleteDatabase, "delete-database", deleteDatabase, "Delete shared database resources")
	fs.BoolVar(&deleteVhost, "delete-vhost", deleteVhost, "Delete virtual host/proxy config")
	fs.BoolVar(&yes, "yes", false, "Confirm deletion")
	fs.StringVar(&confirm, "confirm", "", "Confirm deletion by repeating the deployment name")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("confirm"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: flatrun deployment delete NAME")
		return 2
	}
	if !yes && confirm != fs.Arg(0) {
		fmt.Fprintf(stderr, "Error: refusing to delete %q without --yes or --confirm %s\n", fs.Arg(0), fs.Arg(0))
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.DeleteDeployment(context.Background(), fs.Arg(0), flatrun.DeleteDeploymentOptions{
		DeleteSSL:      deleteSSL,
		DeleteDatabase: deleteDatabase,
		DeleteVhost:    deleteVhost,
	})
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
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
	return runClientCommand(clientCommand{
		name:        "deployment " + kind,
		usage:       fmt.Sprintf("Usage: flatrun deployment %s NAME", kind),
		rawJSON:     true,
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
	}, args, stdout, stderr)
}

func runDeploymentDeploy(args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{}
	operation := "restart"
	pull := true
	onlyLatest := false

	fs := globalFlagSet("deployment deploy", &opts, stderr)
	fs.StringVar(&operation, "operation", operation, "Operation after pull: restart, rebuild, or start")
	fs.BoolVar(&pull, "pull", pull, "Pull images before running the operation")
	fs.BoolVar(&onlyLatest, "only-latest", onlyLatest, "Only pull images tagged latest")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("operation"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: flatrun deployment deploy NAME [--operation restart|rebuild|start]")
		return 2
	}
	if !deploymentOperations[operation] {
		fmt.Fprintln(stderr, "Error: --operation must be restart, rebuild, or start")
		return 2
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.Deploy(context.Background(), fs.Arg(0), flatrun.DeployRequest{
		Action:     operation,
		Pull:       pull,
		OnlyLatest: onlyLatest,
	})
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	printResponse(stdout, opts.JSON, data, "Deployment completed")
	return 0
}

func runImage(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: flatrun image <list|pull|delete>")
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
		fmt.Fprintf(stderr, "Unknown image command: %s\n", args[0])
		return 2
	}
}

func runImageList(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "image list",
		usage:       "Usage: flatrun image list",
		rawJSON:     true,
		positionals: 0,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ListImages(ctx)
		},
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
		fmt.Fprintln(stderr, "Usage: flatrun container <list|start|stop|restart|delete>")
		return 2
	}

	switch args[0] {
	case "list":
		return runContainerList(args[1:], stdout, stderr)
	case "start", "stop", "restart":
		return runContainerSimple(args[0], args[1:], stdout, stderr)
	case "delete":
		return runContainerDelete(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown container command: %s\n", args[0])
		return 2
	}
}

func runContainerList(args []string, stdout, stderr io.Writer) int {
	return runClientCommand(clientCommand{
		name:        "container list",
		usage:       "Usage: flatrun container list",
		rawJSON:     true,
		positionals: 0,
		run: func(ctx context.Context, client *flatrun.Client, args []string) ([]byte, error) {
			return client.ListContainers(ctx)
		},
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
		fmt.Fprintln(stderr, "Usage: flatrun api <get|post|put|delete> PATH [--data JSON]")
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
		fmt.Fprintf(stderr, "Unknown api command: %s\n", args[0])
		return 2
	}
}

func runRawAPI(method string, args []string, stdout, stderr io.Writer) int {
	opts := globalOptions{JSON: true}
	dataArg := ""
	fs := globalFlagSet("api "+strings.ToLower(method), &opts, stderr)
	fs.StringVar(&dataArg, "data", "", "JSON request body")
	if code, ok := parseFlagSet(fs, interspersedFlags(args, globalValueFlags("data"))); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(stderr, "Usage: flatrun api %s /path [--data JSON]\n", strings.ToLower(method))
		return 2
	}

	var payload any
	if dataArg != "" {
		if err := json.Unmarshal([]byte(dataArg), &payload); err != nil {
			fmt.Fprintln(stderr, "Error: invalid --data JSON:", err)
			return 2
		}
	}

	client, err := clientFromOptions(opts)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	data, err := client.Raw(context.Background(), method, normalizeAPIPath(fs.Arg(0)), payload)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
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
		fmt.Fprintln(stdout, string(data))
		return
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		if fallback == "" {
			fmt.Fprintln(stdout, string(data))
			return
		}
		fmt.Fprintln(stdout, fallback)
		return
	}
	if message, ok := response["message"].(string); ok && strings.TrimSpace(message) != "" {
		fmt.Fprintln(stdout, message)
		return
	}
	if status, ok := response["status"].(string); ok && strings.TrimSpace(status) != "" && fallback != "" {
		fmt.Fprintf(stdout, "%s: %s\n", fallback, status)
		return
	}
	fmt.Fprintln(stdout, fallback)
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
