package presentation

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestPlainRendererDoesNotEmitANSI(t *testing.T) {
	var output bytes.Buffer
	renderer := New(&output)
	renderer.Table(&output, []string{"NAME", "STATUS"}, [][]string{{"api", "running"}})
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("plain output contains ANSI: %q", output.String())
	}
	if output.String() != "NAME  STATUS\napi   running\n" {
		t.Fatalf("plain table = %q", output.String())
	}
}

func TestColoredTableFitsConfiguredWidth(t *testing.T) {
	var output bytes.Buffer
	renderer := Renderer{color: true, width: 28}
	renderer.Table(&output, []string{"NAME", "STATUS", "DESCRIPTION"}, [][]string{{"api", "running", "A description that must wrap"}})
	for _, line := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
		if width := lipgloss.Width(line); width > 28 {
			t.Fatalf("line width = %d, line = %q", width, line)
		}
	}
}

func TestColorRendererStylesSemanticOutput(t *testing.T) {
	renderer := Renderer{color: true}
	for _, value := range []string{renderer.Title("FlatRun CLI"), renderer.Status("healthy"), renderer.Status("failed")} {
		if !strings.Contains(value, "\x1b[") {
			t.Fatalf("styled output has no ANSI: %q", value)
		}
	}
}

func TestSemanticWriterPreservesPlainErrors(t *testing.T) {
	var output bytes.Buffer
	writer := SemanticWriter(&output)
	if _, err := writer.Write([]byte("Error: unavailable\n")); err != nil {
		t.Fatal(err)
	}
	if output.String() != "Error: unavailable\n" {
		t.Fatalf("output = %q", output.String())
	}
}
