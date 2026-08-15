package command

import (
	"bytes"
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

// checkFields refuses an unknown or missing field before anything is sent, since a 400 names
// neither the field the agent wanted nor the one it did not understand. It reads the body being
// sent rather than the flags it came from, so --data is held to the same requirements as -f.
func checkFields(api *spec.Spec, op spec.Operation, payload any) error {
	fields := api.Fields(op)
	if len(fields) == 0 {
		return nil
	}
	sent, ok := payload.(map[string]any)
	if payload != nil && !ok {
		// A body that is not an object, such as a bare array, has no fields to check.
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
	if len(missing) > 0 {
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

// closest is the nearest accepted name to what was typed, when one is near enough to have been meant.
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

// renderAnswer lays out a response by the shape the agent answers in, so a list of certificates
// and a list of backups take the same path.
func renderAnswer(w io.Writer, api *spec.Spec, op spec.Operation, data []byte) bool {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(data, &body); err != nil {
		return false
	}

	shape, ok := spec.Shape{}, false
	if api != nil {
		shape, ok = api.Shape(op)
	}
	if !ok {
		// An endpoint the agent has not typed yet, or an agent too old to describe itself. The
		// answer still says which of its fields holds the rows.
		shape, ok = inferShape(body)
		if !ok {
			return false
		}
	}

	switch shape.Kind {
	case "list":
		return renderList(w, shape, body)
	case "item":
		return renderItem(w, shape, body)
	case "message":
		var message struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(data, &message); err != nil || message.Message == "" {
			return false
		}
		_, _ = fmt.Fprintln(w, message.Message)
		return true
	}
	return false
}

// inferShape finds the rows in an answer that nothing described: the one field holding a list.
func inferShape(body map[string]json.RawMessage) (spec.Shape, bool) {
	// The shared list shape carries its rows under items, and the name it used to answer under
	// beside them, so there is no ambiguity to resolve.
	if _, ok := body["items"]; ok {
		return spec.Shape{Kind: "list", Key: "items"}, true
	}

	found := ""
	for name, raw := range body {
		var rows []json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			continue
		}
		if found != "" {
			return spec.Shape{}, false
		}
		found = name
	}
	if found == "" {
		return spec.Shape{}, false
	}
	return spec.Shape{Kind: "list", Key: found}, true
}

func renderList(w io.Writer, shape spec.Shape, body map[string]json.RawMessage) bool {
	raw, ok := body[shape.Key]
	if !ok {
		return false
	}
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return false
	}
	if len(values) == 0 {
		_, _ = fmt.Fprintln(w, "None")
		return true
	}

	// A column heading over a single column of names is furniture. Names print as names.
	rows := make([]map[string]any, 0, len(values))
	for _, value := range values {
		row, ok := value.(map[string]any)
		if !ok {
			for _, value := range values {
				_, _ = fmt.Fprintln(w, cell(value))
			}
			return true
		}
		rows = append(rows, row)
	}

	columns := shape.Columns
	if len(columns) == 0 {
		columns = inferColumns(raw, rows[0])
	}
	if len(columns) == 1 {
		for _, row := range rows {
			_, _ = fmt.Fprintln(w, cell(row[columns[0]]))
		}
		return true
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headings := make([]string, 0, len(columns))
	for _, column := range columns {
		headings = append(headings, strings.ToUpper(strings.ReplaceAll(column, "_", " ")))
	}
	_, _ = fmt.Fprintln(tw, strings.Join(headings, "\t"))
	for _, row := range rows {
		cells := make([]string, 0, len(columns))
		for _, column := range columns {
			cells = append(cells, cell(row[column]))
		}
		_, _ = fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	_ = tw.Flush()
	return true
}

// inferColumns takes the scalar fields of a row, in the order the answer wrote them, since a map
// has no order and alphabetical puts the identifier in the middle.
func inferColumns(raw json.RawMessage, first map[string]any) []string {
	scalar := map[string]bool{}
	for name, value := range first {
		switch value.(type) {
		case map[string]any, []any:
		default:
			scalar[name] = true
		}
	}

	var ordered []string
	decoder := json.NewDecoder(bytes.NewReader(raw))
	depth := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch typed := token.(type) {
		case json.Delim:
			if typed == '{' || typed == '[' {
				depth++
			} else {
				depth--
			}
			if depth < 2 && len(ordered) > 0 {
				// Past the first row, so the order is settled.
				return capColumns(ordered)
			}
		case string:
			if depth == 2 && scalar[typed] {
				ordered = append(ordered, typed)
				scalar[typed] = false
			}
		}
	}
	return capColumns(ordered)
}

func capColumns(columns []string) []string {
	if len(columns) > maxInferredColumns {
		return columns[:maxInferredColumns]
	}
	return columns
}

const maxInferredColumns = 6

func renderItem(w io.Writer, shape spec.Shape, body map[string]json.RawMessage) bool {
	raw, ok := body[shape.Key]
	if !ok {
		return false
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}

	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, name := range names {
		_, _ = fmt.Fprintf(tw, "%s\t%s\n", strings.ReplaceAll(name, "_", " "), cell(fields[name]))
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
