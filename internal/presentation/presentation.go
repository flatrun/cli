package presentation

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"golang.org/x/term"
)

type Renderer struct {
	color bool
	width int
}

type semanticWriter struct {
	writer   io.Writer
	renderer Renderer
}

func SemanticWriter(w io.Writer) io.Writer {
	return semanticWriter{writer: w, renderer: New(w)}
}

func (w semanticWriter) Write(value []byte) (int, error) {
	text := string(value)
	styled := text
	switch {
	case strings.HasPrefix(text, "Error:"), strings.HasPrefix(text, "Unknown command:"):
		styled = w.renderer.Error(strings.TrimSuffix(text, "\n")) + newline(text)
	case strings.HasPrefix(text, "Warning:"):
		styled = w.renderer.Warning(strings.TrimSuffix(text, "\n")) + newline(text)
	}
	_, err := io.WriteString(w.writer, styled)
	if err != nil {
		return 0, err
	}
	return len(value), nil
}

func newline(value string) string {
	if strings.HasSuffix(value, "\n") {
		return "\n"
	}
	return ""
}

func New(w io.Writer) Renderer {
	file, ok := w.(*os.File)
	width := 0
	if ok {
		width, _, _ = term.GetSize(int(file.Fd()))
	}
	return Renderer{color: ok && term.IsTerminal(int(file.Fd())) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb", width: width}
}

func (r Renderer) Title(value string) string {
	return r.render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")), value)
}
func (r Renderer) Heading(value string) string {
	return r.render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")), value)
}
func (r Renderer) Command(value string) string {
	return r.render(lipgloss.NewStyle().Foreground(lipgloss.Color("12")), value)
}
func (r Renderer) Muted(value string) string { return r.render(lipgloss.NewStyle().Faint(true), value) }
func (r Renderer) Success(value string) string {
	return r.render(lipgloss.NewStyle().Foreground(lipgloss.Color("10")), value)
}
func (r Renderer) Warning(value string) string {
	return r.render(lipgloss.NewStyle().Foreground(lipgloss.Color("11")), value)
}
func (r Renderer) Error(value string) string {
	return r.render(lipgloss.NewStyle().Foreground(lipgloss.Color("9")), value)
}

func (r Renderer) Status(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "healthy", "running", "ready", "active", "succeeded", "success", "online", "yes":
		return r.Success(value)
	case "starting", "pending", "warning", "attention", "unknown":
		return r.Warning(value)
	case "unhealthy", "failed", "error", "offline", "stopped", "no":
		return r.Error(value)
	default:
		return value
	}
}

func (r Renderer) Table(w io.Writer, headers []string, rows [][]string) {
	if !r.color {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
		for _, row := range rows {
			_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
		}
		_ = tw.Flush()
		return
	}
	styledRows := make([][]string, len(rows))
	for i, row := range rows {
		styledRows[i] = append([]string(nil), row...)
		for column, header := range headers {
			if column < len(styledRows[i]) && isStatusColumn(header) {
				styledRows[i][column] = r.Status(styledRows[i][column])
			}
		}
	}
	t := table.New().Border(lipgloss.HiddenBorder()).BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).BorderHeader(false).BorderColumn(false).BorderRow(false).Headers(headers...).Rows(styledRows...).StyleFunc(func(row, _ int) lipgloss.Style {
		if row == table.HeaderRow {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).PaddingRight(2)
		}
		return lipgloss.NewStyle().PaddingRight(2)
	})
	rendered := t.Render()
	if r.width > 0 && lipgloss.Width(rendered) > r.width {
		rendered = t.Width(r.width).Render()
	}
	_, _ = fmt.Fprintln(w, rendered)
}

func (r Renderer) render(style lipgloss.Style, value string) string {
	if !r.color {
		return value
	}
	return style.Render(value)
}

func isStatusColumn(header string) bool {
	switch strings.ToUpper(header) {
	case "STATUS", "STATE", "HEALTH", "LATEST", "ACTIVE":
		return true
	default:
		return false
	}
}
