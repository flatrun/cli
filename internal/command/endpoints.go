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
	"time"

	"github.com/flatrun/cli/internal/flatrun"
	"github.com/flatrun/cli/internal/spec"
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
	// Set on the commands written by hand: extra arguments they take beyond the path, and the
	// marker that says the generated table is not the whole story for this one.
	flags  string
	shaped bool
}

func (e endpoint) command() string { return invocation(e) }

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
		if strings.Contains(path, ":"+name) {
			path = strings.Replace(path, ":"+name, url.PathEscape(args[i]), 1)
			continue
		}
		// A wildcard stands for the rest of the path, so its separators are structure and only
		// what sits between them is escaped.
		path = strings.Replace(path, "*"+name, escapeSubPath(args[i]), 1)
	}
	return path, nil
}

func escapeSubPath(value string) string {
	segments := strings.Split(value, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
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
		return listEndpoints(stdout, stderr, family, false)
	}
	switch args[0] {
	case "help", "-h", "--help":
		return listEndpoints(stdout, stderr, family, false)
	case "--json":
		return listEndpoints(stdout, stderr, family, true)
	}

	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
		return explainEndpoint(family, args[0], stdout, stderr)
	}

	e, ok := findEndpoint(family, args[0])
	if !ok {
		_, _ = fmt.Fprintf(stderr, "Unknown %s command: %s\n\n", family, args[0])
		listEndpoints(stderr, stderr, family, false)
		return 2
	}

	fields := fieldValues{}
	query := queryValues{}
	dataArg := ""
	var api *spec.Spec
	var operation spec.Operation
	described := false

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

			// The agent's own description of this endpoint, when it offers one. Checking here
			// turns a 400 with no explanation into a message naming the field.
			api = spec.Load(ctx, client, client.BaseURL())
			if api != nil {
				if op, found := api.Operation(e.method, e.path); found {
					operation = op
					described = true
					if err := checkQuery(api, op, query); err != nil {
						return nil, err
					}
				}
			}

			if len(query) > 0 {
				path += "?" + url.Values(query).Encode()
			}
			payload, err := requestBody(dataArg, fields)
			if err != nil {
				return nil, err
			}
			if described {
				if err := checkFields(api, operation, payload); err != nil {
					return nil, err
				}
			}
			return client.Do(ctx, e.method, path, payload)
		},
		tabular: true,
		render: func(w io.Writer, data []byte) error {
			if renderAnswer(w, api, operation, data) {
				return nil
			}
			printResponse(w, true, data, "")
			return nil
		},
	}
	return runClientCommand(cmd, args[1:], stdout, stderr)
}

// runAliasedEndpoint reaches a plural family's endpoint from its singular name, so the two are
// not different surfaces.
func runAliasedEndpoint(plural, singular string, args []string, stdout, stderr io.Writer) int {
	if _, ok := findEndpoint(plural, args[0]); ok {
		return runEndpoint(plural, args, stdout, stderr)
	}
	_, _ = fmt.Fprintf(stderr, "Unknown %s command: %s\n\n", singular, args[0])
	listEndpoints(stderr, stderr, singular, false)
	return 2
}

func explainEndpoint(family, op string, stdout, stderr io.Writer) int {
	e, ok := findEndpoint(family, op)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "Unknown %s command: %s\n", family, op)
		return 2
	}

	client, err := clientFromOptions(globalOptions{Timeout: 30 * time.Second})
	if err != nil {
		_, _ = fmt.Fprintln(stdout, invocation(e))
		_, _ = fmt.Fprintln(stdout, "\nConnect to an agent to see the fields this takes.")
		return 0
	}

	api := spec.Load(context.Background(), client, client.BaseURL())
	if api == nil {
		_, _ = fmt.Fprintln(stdout, invocation(e))
		_, _ = fmt.Fprintln(stdout, "\nThis agent does not describe its API, so the fields are not known here.")
		return 0
	}
	operation, found := api.Operation(e.method, e.path)
	if !found {
		_, _ = fmt.Fprintln(stdout, invocation(e))
		return 0
	}
	describeEndpoint(stdout, api, e, operation)
	return 0
}

func requestBody(dataArg string, fields fieldValues) (any, error) {
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
	// A write with no body is normal: restarting a deployment carries nothing.
	return nil, nil
}

func argNames(e endpoint) string {
	names := make([]string, 0, len(e.args))
	for _, arg := range e.args {
		names = append(names, strings.ToUpper(arg))
	}
	return strings.Join(names, " ")
}

// listEndpoints prints what can be run, either for one family or for all of them. The JSON form
// exists because a program driving this CLI should not have to parse help text to find out what
// it can call.
func listEndpoints(stdout, stderr io.Writer, family string, asJSON bool) int {
	list := make([]endpoint, 0, len(generatedEndpoints))
	for _, e := range catalogue() {
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
			Shaped  bool     `json:"shaped,omitempty"`
		}
		out := make([]wire, 0, len(list))
		for _, e := range list {
			args := e.args
			if args == nil {
				args = []string{}
			}
			out = append(out, wire{e.family, e.op, e.method, e.path, args, e.command(), e.shaped})
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
		_, _ = fmt.Fprintf(stdout, "  %-38s %s %s\n", strings.TrimSpace(e.op+" "+argNames(e)+" "+e.flags), e.method, e.path)
	}
	_, _ = fmt.Fprintln(stdout)
	_, _ = fmt.Fprintln(stdout, "Send a body with -f name=value (repeatable) or --data JSON.")
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
