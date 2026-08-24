package command

import (
	"context"
	"errors"
	"io"
	"os"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

type operationResult struct {
	data []byte
	err  error
}

type progressModel struct {
	spinner spinner.Model
	label   string
	result  operationResult
	done    bool
	run     func() operationResult
}

func newProgressModel(label string, run func() operationResult) progressModel {
	indicator := spinner.New(spinner.WithSpinner(spinner.Dot))
	indicator.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	return progressModel{spinner: indicator, label: label, run: run}
}

func (m progressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg { return m.run() })
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case operationResult:
		m.result = msg
		m.done = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m progressModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(m.spinner.View() + " " + m.label)
}

func terminalInteraction(w io.Writer, jsonOutput bool) bool {
	if jsonOutput || !stdinIsTerminal() || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

var interactiveSession = terminalInteraction

func runWithProgress(ctx context.Context, w io.Writer, label string, run func(context.Context) ([]byte, error)) ([]byte, error) {
	operationContext, cancel := context.WithCancel(ctx)
	defer cancel()
	model := newProgressModel(label, func() operationResult {
		data, err := run(operationContext)
		return operationResult{data: data, err: err}
	})
	final, err := tea.NewProgram(model, tea.WithInput(stdin), tea.WithOutput(w)).Run()
	if err != nil {
		cancel()
		return nil, err
	}
	result := final.(progressModel).result
	if !final.(progressModel).done {
		cancel()
		return nil, context.Canceled
	}
	return result.data, result.err
}

func runOperation(ctx context.Context, w io.Writer, jsonOutput bool, label string, run func(context.Context) ([]byte, error)) ([]byte, error) {
	if label == "" || !interactiveSession(w, jsonOutput) {
		return run(ctx)
	}
	return runWithProgress(ctx, w, label, run)
}

var promptProfileSetup = func(w io.Writer, profile, urlValue, token *string) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Profile name").Value(profile),
		huh.NewInput().Title("Agent URL").Placeholder("https://panel.example.com").Value(urlValue),
		huh.NewInput().Title("API token").EchoMode(huh.EchoModePassword).Value(token),
	)).WithInput(stdin).WithOutput(w).Run()
}

var promptLoginCredentials = func(w io.Writer, username, password *string) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Username").Value(username),
		huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(password),
	)).WithInput(stdin).WithOutput(w).Run()
}

var promptDeploymentDelete = func(w io.Writer, name string) (bool, error) {
	confirmed := false
	err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Delete deployment " + name + "?").
			Description("Containers and selected deployment resources will be removed.").
			Affirmative("Delete").
			Negative("Cancel").
			Value(&confirmed),
	)).WithInput(stdin).WithOutput(w).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return false, nil
	}
	return confirmed, err
}
