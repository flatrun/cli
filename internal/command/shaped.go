package command

import (
	"io"
	"strings"
)

// shapedCommands are the commands written by hand rather than taken from the route table,
// because they render a table, take flags shaped for the task, or read a command after `--`.
// They are listed here so that one catalogue answers what the CLI can do: a caller reading the
// JSON listing sees them alongside the generated ones instead of only half the surface.
var shapedCommands = []endpoint{
	{family: "deployment", op: "list", method: "GET", path: "/deployments"},
	{family: "deployment", op: "info", method: "GET", path: "/deployments/:name", args: []string{"name"}},
	{family: "deployment", op: "get", method: "GET", path: "/deployments/:name", args: []string{"name"}},
	{family: "deployment", op: "create", method: "POST", path: "/deployments", flags: "--image --port --host-port"},
	{family: "deployment", op: "delete", method: "DELETE", path: "/deployments/:name", args: []string{"name"}},
	{family: "deployment", op: "start", method: "POST", path: "/deployments/:name/manage", args: []string{"name"}},
	{family: "deployment", op: "stop", method: "POST", path: "/deployments/:name/manage", args: []string{"name"}},
	{family: "deployment", op: "restart", method: "POST", path: "/deployments/:name/manage", args: []string{"name"}},
	{family: "deployment", op: "rebuild", method: "POST", path: "/deployments/:name/manage", args: []string{"name"}},
	{family: "deployment", op: "deploy", method: "POST", path: "/deployments/:name/deploy", args: []string{"name"}, flags: "--operation --pull"},
	{family: "deployment", op: "pull", method: "POST", path: "/deployments/:name/pull", args: []string{"name"}, flags: "--only-latest"},
	{family: "deployment", op: "images", method: "GET", path: "/deployments/:name/images", args: []string{"name"}},
	{family: "deployment", op: "containers", method: "GET", path: "/deployments/:name/containers", args: []string{"name"}},
	{family: "deployment", op: "services", method: "GET", path: "/deployments/:name/services", args: []string{"name"}},
	{family: "deployment", op: "actions", method: "GET", path: "/deployments/:name/actions", args: []string{"name"}},
	{family: "deployment", op: "action", method: "POST", path: "/deployments/:name/actions/:actionId", args: []string{"name", "actionId"}},
	{family: "deployment", op: "exec", method: "POST", path: "/deployments/:name/exec", args: []string{"name"}, flags: "[SERVICE] -- COMMAND"},
	{family: "deployment", op: "image set", method: "PUT", path: "/deployments/:name/compose", args: []string{"name", "service", "image"}, flags: "--deploy --operation"},

	{family: "image", op: "list", method: "GET", path: "/images"},
	{family: "image", op: "pull", method: "POST", path: "/images/pull", args: []string{"image"}, flags: "--credential-id"},
	{family: "image", op: "delete", method: "DELETE", path: "/images/:id", args: []string{"id"}},

	{family: "container", op: "list", method: "GET", path: "/containers"},
	{family: "container", op: "start", method: "POST", path: "/containers/:id/start", args: []string{"id"}},
	{family: "container", op: "stop", method: "POST", path: "/containers/:id/stop", args: []string{"id"}},
	{family: "container", op: "restart", method: "POST", path: "/containers/:id/restart", args: []string{"id"}},
	{family: "container", op: "exec", method: "POST", path: "/containers/:id/exec", args: []string{"id"}, flags: "-- COMMAND"},
	{family: "container", op: "delete", method: "DELETE", path: "/containers/:id", args: []string{"id"}},
}

// catalogue is every command the CLI can run: the hand-shaped ones and the generated ones. The
// singular family a hand-shaped command lives under also reaches its plural counterpart, so both
// names appear rather than only the half a reader happened to look under.
func catalogue() []endpoint {
	all := make([]endpoint, 0, len(shapedCommands)+len(generatedEndpoints))
	seen := map[string]bool{}
	for _, e := range shapedCommands {
		e.shaped = true
		all = append(all, e)
		seen[e.family+" "+e.op] = true
	}
	for _, e := range generatedEndpoints {
		if seen[e.family+" "+e.op] {
			continue
		}
		all = append(all, e)
		// A hand-shaped singular family reaches every operation of its plural one.
		if singular, ok := shapedAlias[e.family]; ok && !seen[singular+" "+e.op] {
			alias := e
			alias.family = singular
			all = append(all, alias)
		}
	}
	return all
}

// shapedAlias maps a generated family onto the singular name the hand-shaped commands use.
var shapedAlias = map[string]string{
	"deployments": "deployment",
	"images":      "image",
	"containers":  "container",
}

func invocation(e endpoint) string {
	parts := []string{"flatrun", e.family, e.op}
	for _, arg := range e.args {
		parts = append(parts, strings.ToUpper(arg))
	}
	if e.flags != "" {
		parts = append(parts, e.flags)
	}
	return strings.Join(parts, " ")
}

func shapedCommand(family, op string) bool {
	for _, e := range shapedCommands {
		if e.family == family && e.op == op {
			return true
		}
	}
	return false
}

// runShaped dispatches to the hand-written command, so `containers list` and `container list`
// print the same thing rather than one table and one wall of JSON.
func runShaped(family string, args []string, stdout, stderr io.Writer) int {
	switch family {
	case "deployment":
		return runDeployment(args, stdout, stderr)
	case "image":
		return runImage(args, stdout, stderr)
	case "container":
		return runContainer(args, stdout, stderr)
	}
	return 2
}

// resolveFamily takes whichever way a resource was typed and answers with the one name the CLI
// holds it under. Nobody should have to remember whether the API said backup or backups.
func resolveFamily(typed string) (string, bool) {
	if knownFamily(typed) {
		return typed, true
	}
	if singular, ok := shapedAlias[typed]; ok {
		return singular, true
	}
	for _, candidate := range []string{typed + "s", typed + "es", strings.TrimSuffix(typed, "s")} {
		if candidate != typed && knownFamily(candidate) {
			return candidate, true
		}
	}
	return "", false
}
