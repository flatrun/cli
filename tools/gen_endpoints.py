#!/usr/bin/env python3
"""Regenerate internal/command/endpoints_gen.go from the agent's API description.

    python3 tools/gen_endpoints.py ../agent/internal/api/openapi.json > internal/command/endpoints_gen.go

The agent describes itself, so the commands are named from that description rather than from a
second reading of the agent's source. Whatever the agent serves is what the CLI can reach.
"""

import collections
import json
import sys

WRITE_VERB = {"POST": "create", "PUT": "update", "PATCH": "update", "DELETE": "delete"}

# Reached by the agent's own components, not by an operator.
SKIP_PREFIXES = ("/api/internal", "/api/_internal", "/api/security/events/ingest", "/api/traffic/ingest")

# The description of the API is not a resource of it.
SKIP_PATHS = ("/api/openapi.json",)


def endpoints(spec):
    for path, methods in spec["paths"].items():
        if path.startswith(SKIP_PREFIXES) or path in SKIP_PATHS:
            continue
        for method, operation in methods.items():
            params = [p for p in operation.get("parameters", []) if p["in"] == "path"]
            yield {
                "method": method.upper(),
                # The CLI holds paths as the router writes them, without the /api the client adds.
                "path": path[len("/api"):],
                "args": [p["name"] for p in params],
                "rest": {p["name"] for p in params if p.get("x-rest-of-path")},
            }


def op_name(method, segments):
    literals = [s for s in segments if not s.startswith("{")]
    params = [s for s in segments if s.startswith("{")]
    if not literals:
        if method == "GET":
            return "get" if params else "list"
        return WRITE_VERB[method]
    return "-".join(literals)


def cli_path(endpoint):
    """The path as the router writes it, so a segment holding the rest of the path stays marked."""
    out = []
    for segment in endpoint["path"].strip("/").split("/"):
        if not segment.startswith("{"):
            out.append(segment)
            continue
        name = segment.strip("{}")
        out.append(("*" if name in endpoint["rest"] else ":") + name)
    return "/" + "/".join(out)


def build(spec):
    families = collections.defaultdict(list)
    for endpoint in endpoints(spec):
        segments = endpoint["path"].strip("/").split("/")
        families[segments[0]].append((endpoint, segments[1:]))

    table = []
    for family in sorted(families):
        entries = [[op_name(e["method"], rest), e, rest]
                   for e, rest in sorted(families[family], key=lambda r: (r[0]["path"], r[0]["method"]))]

        # Several endpoints under one noun share a name: the collection and the single item, and
        # the read and the write. The plainest keeps the bare name and the rest say what they do.
        used = collections.Counter(name for name, _, _ in entries)
        methods = collections.defaultdict(set)
        for name, endpoint, _ in entries:
            methods[name].add(endpoint["method"])
        plainest = {}
        for name, endpoint, _ in entries:
            count = len(endpoint["args"])
            if endpoint["method"] == "GET" and count < plainest.get(name, (99,))[0]:
                plainest[name] = (count, endpoint["path"])

        # Taken before any renaming below, which would otherwise change what is being counted.
        fewest_args = {}
        for name, endpoint, _ in entries:
            count = len(endpoint["args"])
            fewest_args[name] = min(fewest_args.get(name, count), count)

        for entry in entries:
            name, endpoint, _ = entry
            if used[name] == 1:
                continue
            count = len(endpoint["args"])
            if len(methods[name]) == 1:
                # The same verb on the collection and on one item. Whichever is safer to type by
                # mistake keeps the bare name: reading the collection, but writing to one item, so
                # "renew DOMAIN" renews one and "renew-all" says what it does.
                fewest = fewest_args[name]
                if endpoint["method"] == "GET":
                    if count > fewest:
                        entry[0] = name + "-get"
                elif count == fewest:
                    entry[0] = name + "-all"
                continue
            if name in plainest and plainest[name][1] == endpoint["path"] and endpoint["method"] == "GET":
                continue
            entry[0] = name + "-" + ("get" if endpoint["method"] == "GET" else WRITE_VERB[endpoint["method"]])

        for name, endpoint, _ in entries:
            table.append({
                "family": family,
                "op": name,
                "method": endpoint["method"],
                "path": cli_path(endpoint),
                "args": endpoint["args"],
            })
    return table


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    if len(args) != 1:
        sys.exit("usage: gen_endpoints.py PATH_TO_OPENAPI_JSON [--json]")
    table = build(json.load(open(args[0])))

    if "--json" in sys.argv:
        print(json.dumps(table, indent=1))
        return

    out = [
        "// Code generated by tools/gen_endpoints.py from the agent's API description. DO NOT EDIT.",
        "",
        "package command",
        "",
        "var generatedEndpoints = []endpoint{",
    ]
    for entry in table:
        args = "nil" if not entry["args"] else "[]string{" + ", ".join('"%s"' % a for a in entry["args"]) + "}"
        out.append('\t{family: "%s", op: "%s", method: "%s", path: "%s", args: %s},'
                   % (entry["family"], entry["op"], entry["method"], entry["path"], args))
    out.append("}")
    print("\n".join(out))


if __name__ == "__main__":
    main()
