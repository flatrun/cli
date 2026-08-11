package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/flatrun/cli/internal/flatrun"
)

// endpoint is one agent API endpoint, reachable as `flatrun FAMILY OP ARGS...`. The table is
// generated from the agent's routes rather than written by hand, so an endpoint the agent adds
// is one regeneration away from being a command instead of a hand-written wrapper that drifts.
type endpoint struct {
	family string
	op     string
	method string
	path   string
	args   []string
}

// command is what an operator types, and what `flatrun commands` prints.
func (e endpoint) command() string {
	parts := []string{"flatrun", e.family, e.op}
	for _, arg := range e.args {
		parts = append(parts, strings.ToUpper(arg))
	}
	return strings.Join(parts, " ")
}

func (e endpoint) writes() bool { return e.method != "GET" }

// resolvePath substitutes the positional arguments into the path parameters.
func (e endpoint) resolvePath(args []string) (string, error) {
	if len(args) != len(e.args) {
		return "", fmt.Errorf("expected %d argument(s): %s", len(e.args), e.command())
	}
	path := e.path
	for i, name := range e.args {
		if args[i] == "" {
			return "", fmt.Errorf("%s cannot be empty: %s", strings.ToUpper(name), e.command())
		}
		path = strings.Replace(path, ":"+name, url.PathEscape(args[i]), 1)
	}
	return path, nil
}

func endpointsByFamily() map[string][]endpoint {
	families := map[string][]endpoint{}
	for _, e := range generatedEndpoints {
		families[e.family] = append(families[e.family], e)
	}
	for _, list := range families {
		sort.Slice(list, func(i, j int) bool { return list[i].op < list[j].op })
	}
	return families
}

func findEndpoint(family, op string) (endpoint, bool) {
	for _, e := range generatedEndpoints {
		if e.family == family && e.op == op {
			return e, true
		}
	}
	return endpoint{}, false
}

func knownFamily(family string) bool {
	for _, e := range generatedEndpoints {
		if e.family == family {
			return true
		}
	}
	return false
}

// fieldValues collects repeated -f name=value pairs into a request body. A value that parses as
// JSON is kept as JSON, so -f enabled=true sends a boolean rather than the word.
type fieldValues map[string]any

func (f fieldValues) String() string { return "" }

func (f fieldValues) Set(raw string) error {
	name, value, found := strings.Cut(raw, "=")
	if !found || name == "" {
		return fmt.Errorf("expected name=value, got %q", raw)
	}
	f[name] = parseFieldValue(value)
	return nil
}

func parseFieldValue(value string) any {
	if value == "" {
		return ""
	}
	switch value {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if n, err := strconv.ParseFloat(value, 64); err == nil {
		return n
	}
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		var nested any
		if err := json.Unmarshal([]byte(value), &nested); err == nil {
			return nested
		}
	}
	return value
}

type queryValues url.Values

func (q queryValues) String() string { return "" }

func (q queryValues) Set(raw string) error {
	name, value, found := strings.Cut(raw, "=")
	if !found || name == "" {
		return fmt.Errorf("expected name=value, got %q", raw)
	}
	url.Values(q).Add(name, value)
	return nil
}

// runEndpoint dispatches `flatrun FAMILY OP ...` against the generated table.
func runEndpoint(family string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printFamily(stdout, family)
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		printFamily(stdout, family)
		return 0
	}

	e, ok := findEndpoint(family, args[0])
	if !ok {
		_, _ = fmt.Fprintf(stderr, "Unknown %s command: %s\n\n", family, args[0])
		printFamily(stderr, family)
		return 2
	}

	fields := fieldValues{}
	query := queryValues{}
	dataArg := ""

	cmd := clientCommand{
		name:        family + " " + e.op,
		usage:       "Usage: " + e.command() + " [-f name=value] [--data JSON] [-q name=value]",
		positionals: len(e.args),
		valueFlags:  []string{"data", "f", "q"},
		flags: func(fs *flag.FlagSet) {
			fs.StringVar(&dataArg, "data", "", "JSON request body, or @file to read one")
			fs.Var(fields, "f", "Request body field as name=value, repeatable")
			fs.Var(query, "q", "Query parameter as name=value, repeatable")
		},
		run: func(ctx context.Context, client *flatrun.Client, positional []string) ([]byte, error) {
			path, err := e.resolvePath(positional)
			if err != nil {
				return nil, err
			}
			if len(query) > 0 {
				path += "?" + url.Values(query).Encode()
			}
			payload, err := requestBody(dataArg, fields, e)
			if err != nil {
				return nil, err
			}
			return client.Do(ctx, e.method, path, payload)
		},
	}
	return runClientCommand(cmd, args[1:], stdout, stderr)
}

func requestBody(dataArg string, fields fieldValues, e endpoint) (any, error) {
	if dataArg != "" && len(fields) > 0 {
		return nil, fmt.Errorf("use --data or -f, not both")
	}
	if dataArg != "" {
		raw := []byte(dataArg)
		if strings.HasPrefix(dataArg, "@") {
			contents, err := os.ReadFile(strings.TrimPrefix(dataArg, "@"))
			if err != nil {
				return nil, err
			}
			raw = contents
		}
		var payload any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("invalid JSON body: %w", err)
		}
		return payload, nil
	}
	if len(fields) > 0 {
		return map[string]any(fields), nil
	}
	if e.writes() {
		// A write with no body is normal here: restarting a deployment or renewing a
		// certificate carries nothing.
		return nil, nil
	}
	return nil, nil
}

func printFamily(w io.Writer, family string) {
	list := endpointsByFamily()[family]
	if len(list) == 0 {
		_, _ = fmt.Fprintf(w, "No commands for %s\n", family)
		return
	}
	_, _ = fmt.Fprintf(w, "flatrun %s\n\n", family)
	for _, e := range list {
		_, _ = fmt.Fprintf(w, "  %-28s %s %s\n", e.op+" "+argNames(e), e.method, e.path)
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Send a body with -f name=value (repeatable) or --data JSON.")
}

func argNames(e endpoint) string {
	names := make([]string, 0, len(e.args))
	for _, arg := range e.args {
		names = append(names, strings.ToUpper(arg))
	}
	return strings.Join(names, " ")
}

// runCommands prints every command the CLI can run. An agent driving this CLI needs to know
// what exists without a human reading the docs, which is what --json is for.
func runCommands(args []string, stdout, stderr io.Writer) int {
	asJSON := false
	family := ""
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case strings.HasPrefix(arg, "-"):
			_, _ = fmt.Fprintf(stderr, "Unknown flag: %s\n", arg)
			return 2
		default:
			family = arg
		}
	}

	list := make([]endpoint, 0, len(generatedEndpoints))
	for _, e := range generatedEndpoints {
		if family == "" || e.family == family {
			list = append(list, e)
		}
	}
	if len(list) == 0 {
		_, _ = fmt.Fprintf(stderr, "Unknown family: %s\n", family)
		return 2
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].family != list[j].family {
			return list[i].family < list[j].family
		}
		return list[i].op < list[j].op
	})

	if asJSON {
		type wire struct {
			Family  string   `json:"family"`
			Op      string   `json:"op"`
			Method  string   `json:"method"`
			Path    string   `json:"path"`
			Args    []string `json:"args"`
			Command string   `json:"command"`
		}
		out := make([]wire, 0, len(list))
		for _, e := range list {
			args := e.args
			if args == nil {
				args = []string{}
			}
			out = append(out, wire{e.family, e.op, e.method, e.path, args, e.command()})
		}
		encoded, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(encoded))
		return 0
	}

	current := ""
	for _, e := range list {
		if e.family != current {
			if current != "" {
				_, _ = fmt.Fprintln(stdout)
			}
			current = e.family
			_, _ = fmt.Fprintln(stdout, e.family)
		}
		_, _ = fmt.Fprintf(stdout, "  %-30s %s %s\n", e.op+" "+argNames(e), e.method, e.path)
	}
	return 0
}

func families() []string {
	seen := map[string]bool{}
	names := []string{}
	for _, e := range generatedEndpoints {
		if !seen[e.family] {
			seen[e.family] = true
			names = append(names, e.family)
		}
	}
	sort.Strings(names)
	return names
}
