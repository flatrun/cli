#!/usr/bin/env python3
"""Regenerate internal/command/endpoints_gen.go from the agent's route table.

    python3 tools/gen_endpoints.py ../agent > internal/command/endpoints_gen.go

The agent registers its routes in internal/api/server.go and publishes no machine-readable
spec, so the routes are read from that file. Adding an endpoint there and rerunning this is all
the CLI needs to reach it.
"""

import collections
import json
import re
import sys

GROUP_PREFIX = {
    "api": "",
    "protected": "",
    "setupGroup": "/setup",
    "guarded": "/setup",
    "usersGroup": "/users",
    "apiKeysGroup": "/apikeys",
    "dnsGroup": "/dns",
    "clusterGroup": "/cluster",
}

# Reached by the agent's own plugins and by nginx, never by an operator.
SKIP_PREFIXES = ("/internal", "/_internal", "/security/events/ingest", "/traffic/ingest")

# Streaming endpoints: a websocket or a long-lived follow, which the table's request/response
# shape cannot carry.
SKIP_SUFFIXES = ("/stream", "/ws", "/terminal/interactive", "/exec/interactive")

WRITE_VERB = {"POST": "create", "PUT": "update", "PATCH": "update", "DELETE": "delete"}


def routes(agent_path):
    src = open(agent_path + "/internal/api/server.go").read()
    pattern = re.compile(r'\b(\w+)\.(GET|POST|PUT|DELETE|PATCH)\(\s*"([^"]+)"(.*?)\)\s*$', re.M)
    for match in pattern.finditer(src):
        group, method, path, rest = match.groups()
        if group not in GROUP_PREFIX:
            continue
        full = GROUP_PREFIX[group] + path
        if full.startswith(SKIP_PREFIXES) or full.endswith(SKIP_SUFFIXES):
            continue
        perm = re.search(r"auth\.(Perm\w+)", rest)
        yield {"method": method, "path": full, "perm": perm.group(1) if perm else ""}


def op_name(method, segments):
    literals = [s for s in segments if not s.startswith(":")]
    params = [s for s in segments if s.startswith(":")]
    if not literals:
        if method == "GET":
            return "get" if params else "list"
        return WRITE_VERB[method]
    name = "-".join(literals)
    return name


def build(agent_path):
    families = collections.defaultdict(list)
    for route in routes(agent_path):
        segments = route["path"].strip("/").split("/")
        family, rest = segments[0], segments[1:]
        families[family].append((route, rest))

    table = []
    for family in sorted(families):
        used = collections.Counter()
        entries = []
        for route, rest in sorted(families[family], key=lambda r: (r[0]["path"], r[0]["method"])):
            name = op_name(route["method"], rest)
            entries.append([name, route, rest])
        # Several endpoints under one noun share a name: the collection and the single item,
        # and the read and the write. The plainest one keeps the bare name and the rest say what
        # they do, so "domains" lists them and "domains-delete" removes one.
        for name, _, _ in entries:
            used[name] += 1
        plainest = {}
        for name, route, rest in entries:
            arg_count = sum(1 for s in rest if s.startswith(":"))
            if route["method"] == "GET" and arg_count < plainest.get(name, (99,))[0]:
                plainest[name] = (arg_count, route["path"])
        methods = collections.defaultdict(set)
        for name, route, _ in entries:
            methods[name].add(route["method"])
        for entry in entries:
            name, route, rest = entry
            if used[name] == 1:
                continue
            arg_count = sum(1 for s in rest if s.startswith(":"))
            if len(methods[name]) == 1:
                # The same verb on the collection and on one item. Whichever is safer to type by
                # mistake keeps the bare name: reading the collection, but writing to one item,
                # so "renew DOMAIN" renews one and "renew-all" says what it does.
                fewest = min(sum(1 for s in r.strip("/").split("/") if s.startswith(":"))
                             for n, rt, r in [(n, rt, rt["path"]) for n, rt, _ in entries if n == name])
                if route["method"] == "GET":
                    if arg_count > fewest:
                        entry[0] = name + "-get"
                elif arg_count == fewest:
                    entry[0] = name + "-all"
                continue
            if name in plainest and plainest[name][1] == route["path"] and route["method"] == "GET":
                continue
            entry[0] = name + "-" + ("get" if route["method"] == "GET" else WRITE_VERB[route["method"]])
        for name, route, rest in entries:
            args = [s.lstrip(":") for s in route["path"].strip("/").split("/") if s.startswith(":")]
            table.append({
                "family": family,
                "op": name,
                "method": route["method"],
                "path": route["path"],
                "args": args,
                "perm": route["perm"],
            })
    return table


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    if len(args) != 1:
        sys.exit("usage: gen_endpoints.py PATH_TO_AGENT_CHECKOUT [--json]")
    table = build(args[0])
    if "--json" in sys.argv:
        print(json.dumps(table, indent=1))
        return

    out = []
    out.append("// Code generated by tools/gen_endpoints.py from the agent's route table. DO NOT EDIT.")
    out.append("")
    out.append("package command")
    out.append("")
    out.append("var generatedEndpoints = []endpoint{")
    for e in table:
        args = "nil" if not e["args"] else "[]string{" + ", ".join('"%s"' % a for a in e["args"]) + "}"
        out.append('\t{family: "%s", op: "%s", method: "%s", path: "%s", args: %s},'
                   % (e["family"], e["op"], e["method"], e["path"], args))
    out.append("}")
    print("\n".join(out))


if __name__ == "__main__":
    main()
