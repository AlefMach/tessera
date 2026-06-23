package plain

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/alef-mach/tessera/internal/event"
)

type Renderer struct {
	in         *bufio.Reader
	out        io.Writer
	styles     styles
	diffStyles diffStyles
	markdown   *glamour.TermRenderer
}

type styles struct {
	title   lipgloss.Style
	label   lipgloss.Style
	muted   lipgloss.Style
	success lipgloss.Style
	warn    lipgloss.Style
	err     lipgloss.Style
	prompt  lipgloss.Style
	code    lipgloss.Style
}

func NewRenderer() *Renderer {
	return NewRendererWithIO(os.Stdin, os.Stdout)
}

func NewRendererWithIO(in io.Reader, out io.Writer) *Renderer {
	width := 100
	if file, ok := out.(*os.File); ok {
		if terminalWidth, _, err := term.GetSize(int(file.Fd())); err == nil && terminalWidth > 20 {
			width = terminalWidth
		}
	}

	md, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)

	return &Renderer{
		in:         bufio.NewReader(in),
		out:        out,
		styles:     newStyles(),
		diffStyles: newDiffStyles(),
		markdown:   md,
	}
}

func newStyles() styles {
	return styles{
		title:   lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
		label:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		success: lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		err:     lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		prompt:  lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
		code:    lipgloss.NewStyle().Foreground(lipgloss.Color("229")),
	}
}

func (r *Renderer) RenderEvent(evt event.Event) {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}

	switch normalizeEventType(evt.Type) {
	case "session.started", "session.resumed":
		r.renderSession(evt)
	case "project.profiled":
		r.renderProjectProfiled(evt)
	case "index.started", "index.finished":
		r.renderSimple(evt)
	case "llm.call.started", "llm.call.finished":
		r.renderLLMCall(evt)
	case "command.proposed":
		r.renderCommandProposed(evt)
	case "approval.requested", "workspace.trust.requested":
		r.renderApprovalRequested(evt)
	case "patch.proposed":
		r.renderPatchProposed(evt)
	case "test.started", "test.finished":
		r.renderTest(evt)
	case "run.finished", "session.ended":
		r.renderRunFinished(evt)
	case "run.aborted":
		r.renderRunAborted(evt)
	case "error.occurred":
		r.renderError(evt)
	default:
		r.renderSimple(evt)
	}
}

func (r *Renderer) ReadLine(prompt string) (string, error) {
	fmt.Fprint(r.out, r.styles.prompt.Render(prompt))
	return r.in.ReadString('\n')
}

func (r *Renderer) renderSession(evt event.Event) {
	title := "Session started"
	if normalizeEventType(evt.Type) == "session.resumed" {
		title = "Session resumed"
	}
	if evt.Title != "" {
		title = evt.Title
	}

	r.writeTitle("◆", title, evt.Timestamp)
	rows := []string{
		kv("session", dataString(evt.Data, "session_id", "session")),
		kv("model", modelLabel(evt.Data)),
		kv("context", contextLabel(evt.Data)),
		kv("calls", firstNonEmpty(dataString(evt.Data, "calls", "llm_calls"), "0")),
		kv("cwd", dataString(evt.Data, "cwd", "project")),
	}
	r.writeRows(rows)
	r.writeMarkdown(evt.Message)
	r.blank()
}

func (r *Renderer) renderProjectProfiled(evt event.Event) {
	r.writeTitle("◇", titleOr(evt, "Project profiled"), evt.Timestamp)
	rows := []string{
		kv("root", dataString(evt.Data, "root", "cwd", "project")),
		kv("language", dataString(evt.Data, "language", "languages")),
		kv("files", dataString(evt.Data, "files", "file_count")),
		kv("tests", dataString(evt.Data, "tests", "test_command")),
		kv("git", dataString(evt.Data, "git", "git_status")),
	}
	r.writeRows(rows)
	r.writeMarkdown(evt.Message)
	r.blank()
}

func (r *Renderer) renderLLMCall(evt event.Event) {
	r.writeTitle("◌", titleOr(evt, "LLM call"), evt.Timestamp)
	rows := []string{
		kv("model", modelLabel(evt.Data)),
		kv("input", tokenLabel(evt.Data, "input_tokens", "prompt_tokens")),
		kv("output", tokenLabel(evt.Data, "output_tokens", "completion_tokens")),
		kv("duration", dataString(evt.Data, "duration", "elapsed")),
	}
	r.writeRows(rows)
	r.writeMarkdown(evt.Message)
	r.blank()
}

func (r *Renderer) renderCommandProposed(evt event.Event) {
	r.writeTitle("$", titleOr(evt, "Command proposed"), evt.Timestamp)
	r.writeMarkdown(evt.Message)
	command := dataString(evt.Data, "command", "cmd")
	if command != "" {
		r.writeBlock(r.styles.code.Render(command))
	}
	r.blank()
}

func (r *Renderer) renderApprovalRequested(evt event.Event) {
	r.writeTitle("?", titleOr(evt, "Approval requested"), evt.Timestamp)
	r.writeMarkdown(evt.Message)
	r.writeKnownData(evt.Data, "risk", "reason", "cwd", "trust_store")
	r.blank()
}

func (r *Renderer) renderPatchProposed(evt event.Event) {
	r.writeTitle("±", titleOr(evt, "Patch proposed"), evt.Timestamp)
	r.writeMarkdown(evt.Message)
	r.writeKnownData(evt.Data, "files", "summary")
	if diff := dataString(evt.Data, "diff", "patch"); diff != "" {
		r.writeBlock(renderDiff(diff, r.diffStyles))
	}
	r.blank()
}

func (r *Renderer) renderTest(evt event.Event) {
	style := r.styles.title
	if normalizeEventType(evt.Type) == "test.finished" && !eventSucceeded(evt.Data) {
		style = r.styles.warn
	}
	r.writeTitleWithStyle("✓", titleOr(evt, "Test"), evt.Timestamp, style)
	r.writeKnownData(evt.Data, "command", "status", "exit_code", "duration")
	r.writeMarkdown(evt.Message)
	if output := dataString(evt.Data, "output", "stdout", "stderr"); output != "" {
		r.writeBlock(output)
	}
	r.blank()
}

func (r *Renderer) renderRunFinished(evt event.Event) {
	r.writeTitleWithStyle("✓", titleOr(evt, "Run finished"), evt.Timestamp, r.styles.success)
	r.writeKnownData(evt.Data, "status", "duration", "llm_calls", "commands", "patches", "tests")
	r.writeMarkdown(evt.Message)
	r.blank()
}

func (r *Renderer) renderRunAborted(evt event.Event) {
	r.writeTitleWithStyle("!", titleOr(evt, "Run aborted"), evt.Timestamp, r.styles.warn)
	r.writeKnownData(evt.Data, "reason", "status", "duration")
	r.writeMarkdown(evt.Message)
	r.blank()
}

func (r *Renderer) renderError(evt event.Event) {
	r.writeTitleWithStyle("!", titleOr(evt, "Error"), evt.Timestamp, r.styles.err)
	r.writeKnownData(evt.Data, "error", "cause", "file", "line")
	r.writeMarkdown(evt.Message)
	r.blank()
}

func (r *Renderer) renderSimple(evt event.Event) {
	r.writeTitle("●", titleOr(evt, evt.Type), evt.Timestamp)
	r.writeMarkdown(evt.Message)
	r.writeExtraData(evt.Data)
	r.blank()
}

func (r *Renderer) writeTitle(mark, title string, ts time.Time) {
	r.writeTitleWithStyle(mark, title, ts, r.styles.title)
}

func (r *Renderer) writeTitleWithStyle(mark, title string, ts time.Time, style lipgloss.Style) {
	timeLabel := ts.Local().Format("15:04:05")
	fmt.Fprintf(r.out, "%s %s %s\n", style.Render(mark), style.Render(title), r.styles.muted.Render(timeLabel))
}

func (r *Renderer) writeRows(rows []string) {
	for _, row := range rows {
		if row != "" {
			fmt.Fprintln(r.out, row)
		}
	}
}

func (r *Renderer) writeKnownData(data map[string]any, keys ...string) {
	rows := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := dataString(data, key); value != "" {
			rows = append(rows, kv(labelFor(key), value))
		}
	}
	r.writeRows(rows)
}

func (r *Renderer) writeExtraData(data map[string]any) {
	if len(data) == 0 {
		return
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := dataString(data, key)
		if value != "" {
			fmt.Fprintln(r.out, kv(labelFor(key), value))
		}
	}
}

func (r *Renderer) writeMarkdown(text string) {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return
	}
	if r.markdown != nil {
		if rendered, err := r.markdown.Render(text); err == nil {
			fmt.Fprint(r.out, strings.TrimRight(rendered, "\n")+"\n")
			return
		}
	}
	fmt.Fprintln(r.out, text)
}

func (r *Renderer) writeBlock(text string) {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintln(r.out, line)
	}
}

func (r *Renderer) blank() {
	fmt.Fprintln(r.out)
}

func kv(key, value string) string {
	if value == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  "+key+": ") + value
}

func titleOr(evt event.Event, fallback string) string {
	if evt.Title != "" {
		return evt.Title
	}
	return fallback
}

func normalizeEventType(eventType string) string {
	return strings.NewReplacer("_", ".", "-", ".").Replace(strings.ToLower(eventType))
}

func dataString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if data == nil {
			continue
		}
		value, ok := data[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		case fmt.Stringer:
			return typed.String()
		case []string:
			return strings.Join(typed, ", ")
		case []any:
			parts := make([]string, 0, len(typed))
			for _, item := range typed {
				parts = append(parts, fmt.Sprint(item))
			}
			return strings.Join(parts, ", ")
		default:
			return fmt.Sprint(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func modelLabel(data map[string]any) string {
	provider := dataString(data, "provider")
	model := dataString(data, "model")
	switch {
	case provider != "" && model != "":
		return provider + "/" + model
	case model != "":
		return model
	default:
		return provider
	}
}

func contextLabel(data map[string]any) string {
	context := tokenLabel(data, "context_tokens", "context")
	max := tokenLabel(data, "max_tokens")
	switch {
	case context != "" && max != "":
		return context + " context, " + max + " max"
	case context != "":
		return context
	default:
		return max
	}
}

func tokenLabel(data map[string]any, keys ...string) string {
	value := dataString(data, keys...)
	if value == "" {
		return ""
	}
	if _, err := strconv.Atoi(value); err == nil {
		return value + " tokens"
	}
	return value
}

func eventSucceeded(data map[string]any) bool {
	status := strings.ToLower(dataString(data, "status", "result"))
	if status == "" {
		exitCode := dataString(data, "exit_code", "code")
		return exitCode == "" || exitCode == "0"
	}
	return status == "ok" || status == "pass" || status == "passed" || status == "success" || status == "succeeded"
}

func labelFor(key string) string {
	return strings.ReplaceAll(key, "_", " ")
}
