package command

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/flatrun/cli/internal/spec"
)

// checkFields refuses a body field the endpoint does not accept, and a required one that is
// missing, before anything is sent. The agent would refuse both, but a 400 from a server names
// neither the field it wanted nor the one it did not understand.
func checkFields(api *spec.Spec, op spec.Operation, sent fieldValues) error {
	fields := api.Fields(op)
	if len(fields) == 0 {
		return nil
	}

	known := make(map[string]bool, len(fields))
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		known[field.Name] = true
		names = append(names, field.Name)
	}

	var unknown []string
	for name := range sent {
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		message := fmt.Sprintf("unknown field %s", strings.Join(unknown, ", "))
		if suggestion := closest(unknown[0], names); suggestion != "" {
			message += fmt.Sprintf(". Did you mean %s?", suggestion)
		} else {
			message += fmt.Sprintf(". This endpoint takes: %s", strings.Join(names, ", "))
		}
		return fmt.Errorf("%s", message)
	}

	var missing []string
	for _, field := range fields {
		if field.Required {
			if _, ok := sent[field.Name]; !ok {
				missing = append(missing, field.Name)
			}
		}
	}
	if len(missing) > 0 && len(sent) > 0 {
		return fmt.Errorf("missing required field %s", strings.Join(missing, ", "))
	}
	return nil
}

func checkQuery(api *spec.Spec, op spec.Operation, sent queryValues) error {
	accepted := api.QueryParams(op)
	if len(accepted) == 0 || len(sent) == 0 {
		return nil
	}
	known := make(map[string]bool, len(accepted))
	for _, name := range accepted {
		known[name] = true
	}
	for name := range sent {
		if known[name] {
			continue
		}
		message := fmt.Sprintf("unknown query parameter %s", name)
		if suggestion := closest(name, accepted); suggestion != "" {
			return fmt.Errorf("%s. Did you mean %s?", message, suggestion)
		}
		return fmt.Errorf("%s. This endpoint reads: %s", message, strings.Join(accepted, ", "))
	}
	return nil
}

// closest is the nearest accepted name to what was typed, when one is near enough that it was
// probably meant.
func closest(typed string, candidates []string) string {
	best, bestDistance := "", len(typed)/2+1
	for _, candidate := range candidates {
		if d := distance(typed, candidate); d <= bestDistance {
			best, bestDistance = candidate, d
		}
	}
	return best
}

func distance(a, b string) int {
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, min(current[j-1]+1, previous[j-1]+cost))
		}
		copy(previous, current)
	}
	return previous[len(b)]
}

// describeEndpoint prints what an endpoint takes, which is the answer to "what do I put in -f".
func describeEndpoint(w io.Writer, api *spec.Spec, e endpoint, op spec.Operation) {
	_, _ = fmt.Fprintln(w, invocation(e))
	if op.Permission != "" {
		_, _ = fmt.Fprintf(w, "Needs %s\n", op.Permission)
	}

	if fields := api.Fields(op); len(fields) > 0 {
		_, _ = fmt.Fprintln(w, "\nFields, given as -f name=value:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, field := range fields {
			required := ""
			if field.Required {
				required = "required"
			}
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", field.Name, field.Type, required, field.Help)
		}
		_ = tw.Flush()
	}

	if query := api.QueryParams(op); len(query) > 0 {
		_, _ = fmt.Fprintf(w, "\nQuery parameters, given as -q name=value:\n  %s\n", strings.Join(query, ", "))
	}
}

// renderTable prints an endpoint's answer as columns when the type it returns says which fields
// make a row. Nothing here is written per endpoint: the layout comes from the description.
func renderTable(w io.Writer, api *spec.Spec, op spec.Operation, data []byte) bool {
	table, ok := api.Table(op)
	if !ok {
		return false
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(data, &body); err != nil {
		return false
	}
	raw, ok := body[table.Key]
	if !ok {
		return false
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return false
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintf(w, "No %s\n", table.Key)
		return true
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headings := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		headings = append(headings, strings.ToUpper(strings.ReplaceAll(column, "_", " ")))
	}
	_, _ = fmt.Fprintln(tw, strings.Join(headings, "\t"))
	for _, row := range rows {
		cells := make([]string, 0, len(table.Columns))
		for _, column := range table.Columns {
			cells = append(cells, cell(row[column]))
		}
		_, _ = fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	_ = tw.Flush()
	return true
}

func cell(value any) string {
	switch typed := value.(type) {
	case nil:
		return "-"
	case string:
		if at, err := time.Parse(time.RFC3339, typed); err == nil {
			return at.Local().Format("2006-01-02 15:04")
		}
		return typed
	case bool:
		if typed {
			return "yes"
		}
		return "no"
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', 2, 64)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, cell(item))
		}
		return strings.Join(parts, ",")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "-"
	}
	return string(encoded)
}
