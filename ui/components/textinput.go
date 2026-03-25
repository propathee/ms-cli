package components

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	rw "github.com/mattn/go-runewidth"
	"github.com/vigo999/ms-cli/ui/slash"
)

var (
	sugCmdStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	sugDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	sugSelCmdStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	sugSelDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
)

const maxVisibleSuggestions = 8
const maxVisibleInputRows = 6
const composerPrompt = "❯ "

// TextInput wraps the bubbles textarea for the chat prompt.
type TextInput struct {
	Model            textarea.Model
	slashRegistry    *slash.Registry
	showSuggestions  bool
	slashMode        bool // true once suggestions have been shown, until submit/esc
	slashDismissed   bool
	suggestions      []string
	selectedIdx      int
	suggestionOffset int
	history          []string
	historyIndex     int
	historyDraft     string
}

// NewTextInput creates a focused composer with "? " prompt.
func NewTextInput() TextInput {
	ti := textarea.New()
	ti.Prompt = composerPrompt
	ti.Placeholder = ""
	ti.CharLimit = 2000
	ti.ShowLineNumbers = false
	ti.SetPromptFunc(lipgloss.Width(composerPrompt), func(lineIdx int) string {
		if lineIdx == 0 {
			return composerPrompt
		}
		return strings.Repeat(" ", lipgloss.Width(composerPrompt))
	})
	focused, blurred := textarea.DefaultStyles()
	focused.CursorLine = lipgloss.NewStyle()
	blurred.CursorLine = lipgloss.NewStyle()
	ti.FocusedStyle = focused
	ti.BlurredStyle = blurred
	ti.Focus()
	ti.SetHeight(1)
	return TextInput{
		Model:         ti,
		slashRegistry: slash.DefaultRegistry,
		historyIndex:  -1,
	}
}

// Value returns the current input text.
func (t TextInput) Value() string {
	return t.Model.Value()
}

// InsertText inserts text into the composer while keeping layout and suggestions in sync.
func (t TextInput) InsertText(text string) TextInput {
	t.Model.InsertString(text)
	t.syncLayout()
	t.updateSuggestions()
	return t
}

// Reset clears the input.
func (t TextInput) Reset() TextInput {
	t.Model.Reset()
	t.syncLayout()
	t.showSuggestions = false
	t.slashDismissed = false
	// Keep slashMode бк it gets cleared when the command result arrives.
	t.suggestions = nil
	t.selectedIdx = 0
	t.suggestionOffset = 0
	t.historyIndex = -1
	t.historyDraft = ""
	return t
}

// Focus gives the input focus.
func (t TextInput) Focus() (TextInput, tea.Cmd) {
	cmd := t.Model.Focus()
	t.syncLayout()
	return t, cmd
}

// Blur removes focus from the input.
func (t TextInput) Blur() TextInput {
	t.Model.Blur()
	return t
}

// SetWidth updates the rendered input width.
func (t TextInput) SetWidth(width int) TextInput {
	if width < 1 {
		width = 1
	}
	t.Model.SetWidth(width)
	t.syncLayout()
	return t
}

// Update handles key events.
func (t TextInput) Update(msg tea.Msg) (TextInput, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if t.historyIndex != -1 && keyEditsInput(msg) {
			t.historyIndex = -1
		}
		if keyEditsInput(msg) {
			t.slashDismissed = false
		}

		// Handle slash command suggestions navigation
		if t.showSuggestions && len(t.suggestions) > 0 {
			switch msg.String() {
			case "up":
				if t.selectedIdx > 0 {
					t.selectedIdx--
				} else {
					// Wrap to last
					t.selectedIdx = len(t.suggestions) - 1
				}
				t.syncSuggestionWindow()
				return t, nil
			case "down":
				if t.selectedIdx < len(t.suggestions)-1 {
					t.selectedIdx++
				} else {
					// Wrap to first
					t.selectedIdx = 0
				}
				t.syncSuggestionWindow()
				return t, nil
			case "tab":
				// Accept selected suggestion
				if t.selectedIdx < len(t.suggestions) {
					val := t.suggestions[t.selectedIdx] + " "
					t.Model.SetValue(val)
					t.Model.SetCursor(len([]rune(val)))
					t.syncLayout()
					t.showSuggestions = false
					t.slashMode = false
					t.suggestions = nil
					t.suggestionOffset = 0
				}
				return t, nil
			case "enter":
				// Let the app decide whether enter should submit the current slash command.
				return t, nil
			case "esc":
				// Cancel suggestions
				t.showSuggestions = false
				t.slashMode = false
				t.slashDismissed = true
				t.suggestions = nil
				t.suggestionOffset = 0
				return t, nil
			}
		}
	}

	m, cmd := t.Model.Update(msg)
	t.Model = m
	t.syncLayout()

	// Update suggestions based on current input
	t.updateSuggestions()

	return t, cmd
}

// PushHistory stores a submitted input line for later recall.
func (t TextInput) PushHistory(value string) TextInput {
	value = strings.TrimSpace(value)
	if value == "" {
		return t
	}
	if n := len(t.history); n > 0 && t.history[n-1] == value {
		t.historyIndex = -1
		t.historyDraft = ""
		return t
	}
	t.history = append(t.history, value)
	t.historyIndex = -1
	t.historyDraft = ""
	return t
}

// PrevHistory recalls the previous submitted line.
func (t TextInput) PrevHistory() TextInput {
	if len(t.history) == 0 {
		return t
	}
	if t.historyIndex == -1 {
		t.historyDraft = t.Model.Value()
		t.historyIndex = len(t.history) - 1
	} else if t.historyIndex > 0 {
		t.historyIndex--
	} else {
		return t
	}
	return t.loadHistoryEntry(t.history[t.historyIndex])
}

// NextHistory moves forward in submitted-line history, restoring the draft at the end.
func (t TextInput) NextHistory() TextInput {
	if len(t.history) == 0 || t.historyIndex == -1 {
		return t
	}
	if t.historyIndex < len(t.history)-1 {
		t.historyIndex++
		return t.loadHistoryEntryAtEnd(t.history[t.historyIndex])
	}
	t.historyIndex = -1
	t.Model.SetValue(t.historyDraft)
	t.Model.SetCursor(len([]rune(t.historyDraft)))
	t.syncLayout()
	t.historyDraft = ""
	t.showSuggestions = false
	t.slashMode = false
	t.suggestions = nil
	t.suggestionOffset = 0
	return t
}

// MoveUp applies Codex-style up-arrow navigation inside the composer.
func (t TextInput) MoveUp() TextInput {
	if t.isAtTopDisplayRow() {
		if !t.isAtLineStart() {
			t.Model.CursorStart()
			return t
		}
		return t.PrevHistory()
	}
	if t.visibleInputRows() <= 1 {
		if !t.isAtLineStart() {
			t.Model.CursorStart()
			return t
		}
		return t.PrevHistory()
	}
	t.Model.CursorUp()
	return t
}

// MoveDown applies ClaudeCode-style down-arrow navigation inside the composer.
func (t TextInput) MoveDown() TextInput {
	if t.isAtBottomDisplayRow() {
		if !t.isAtLineEnd() {
			t.Model.CursorEnd()
			return t
		}
		return t.NextHistory()
	}
	if t.visibleInputRows() <= 1 {
		if !t.isAtLineEnd() {
			t.Model.CursorEnd()
			return t
		}
		return t.NextHistory()
	}
	t.Model.CursorDown()
	return t
}

// Paste reads clipboard content through the textarea paste command.
func (t TextInput) Paste() (TextInput, tea.Cmd) {
	return t.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
}

func (t TextInput) isAtTopDisplayRow() bool {
	return t.Model.Line() == 0 && t.Model.LineInfo().RowOffset == 0
}

func (t TextInput) isAtBottomDisplayRow() bool {
	lines := strings.Split(t.Model.Value(), "\n")
	if len(lines) == 0 {
		return true
	}
	info := t.Model.LineInfo()
	return t.Model.Line() == len(lines)-1 && info.RowOffset >= info.Height-1
}

func (t TextInput) isAtLineStart() bool {
	return t.cursorColumn() == 0
}

func (t TextInput) isAtLineEnd() bool {
	row := t.Model.Line()
	lines := strings.Split(t.Model.Value(), "\n")
	if row < 0 || row >= len(lines) {
		return true
	}
	return t.cursorColumn() >= len([]rune(lines[row]))
}

// ResolvedSubmitValue returns the input value normalized for submission.
// When slash suggestions are active, enter submits the currently selected command.
func (t TextInput) ResolvedSubmitValue() string {
	val := strings.TrimSpace(t.Model.Value())
	if !t.showSuggestions || len(t.suggestions) == 0 {
		return val
	}

	selected := t.SelectedSuggestion()
	if selected == "" {
		return val
	}

	command, suffix := splitSlashCommandAndSuffix(val)
	if command == "" || !strings.HasPrefix(selected, command) {
		return val
	}
	return selected + suffix
}

// ShouldInsertNewlineOnEnter reports whether enter should consume an immediate trailing backslash.
func (t TextInput) ShouldInsertNewlineOnEnter() bool {
	row := t.Model.Line()
	lines := strings.Split(t.Model.Value(), "\n")
	if row < 0 || row >= len(lines) {
		return false
	}
	col := t.cursorColumn()
	line := []rune(lines[row])
	if col <= 0 || col > len(line) {
		return false
	}
	return line[col-1] == '\\'
}

// InsertContinuationNewline consumes the immediate trailing backslash and inserts a real newline.
func (t TextInput) InsertContinuationNewline() (TextInput, tea.Cmd) {
	var cmds []tea.Cmd
	m, cmd := t.Model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	t.Model = m
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	t.Model.InsertString("\n")
	t.syncLayout()
	t.updateSuggestions()
	return t, tea.Batch(cmds...)
}

func (t TextInput) loadHistoryEntry(value string) TextInput {
	t.Model.SetValue(value)
	t.moveCursorToStart()
	return t.clearSuggestionsAndSync()
}

func (t TextInput) loadHistoryEntryAtEnd(value string) TextInput {
	t.Model.SetValue(value)
	return t.clearSuggestionsAndSync()
}

func (t TextInput) clearSuggestionsAndSync() TextInput {
	t.syncLayout()
	t.showSuggestions = false
	t.slashMode = false
	t.slashDismissed = false
	t.suggestions = nil
	t.suggestionOffset = 0
	return t
}

func (t *TextInput) moveCursorToStart() {
	for t.Model.Line() > 0 || t.Model.LineInfo().RowOffset > 0 {
		t.Model.CursorUp()
	}
	t.Model.CursorStart()
}

// updateSuggestions updates the slash command suggestions based on current input.
func (t *TextInput) updateSuggestions() {
	val := strings.TrimLeftFunc(t.Model.Value(), unicode.IsSpace)
	selected := t.SelectedSuggestion()

	if t.historyIndex != -1 {
		t.showSuggestions = false
		t.slashMode = false
		t.suggestions = nil
		t.selectedIdx = 0
		t.suggestionOffset = 0
		return
	}

	// Only show suggestions if input starts with "/"
	if !strings.HasPrefix(val, "/") {
		t.showSuggestions = false
		t.slashMode = false
		t.slashDismissed = false
		t.suggestions = nil
		t.selectedIdx = 0
		t.suggestionOffset = 0
		return
	}

	if t.slashDismissed {
		t.showSuggestions = false
		t.slashMode = false
		t.suggestions = nil
		t.selectedIdx = 0
		t.suggestionOffset = 0
		return
	}

	// Get suggestions
	command, suffix := splitSlashCommandAndSuffix(val)
	if suffix != "" {
		t.showSuggestions = false
		t.slashMode = false
		t.suggestions = nil
		t.selectedIdx = 0
		t.suggestionOffset = 0
		return
	}
	t.suggestions = t.slashRegistry.Suggestions(command)
	t.showSuggestions = len(t.suggestions) > 0
	if t.showSuggestions {
		t.slashMode = true
	}

	if selected != "" {
		for i, suggestion := range t.suggestions {
			if suggestion == selected {
				t.selectedIdx = i
				break
			}
		}
	}

	// Reset selection if it's out of bounds.
	if t.selectedIdx >= len(t.suggestions) {
		t.selectedIdx = 0
	}
	if len(t.suggestions) == 0 {
		t.suggestionOffset = 0
		return
	}
	t.syncSuggestionWindow()
}

// View renders the input with optional suggestions.
func (t TextInput) View() string {
	t.syncLayout()
	inputView := t.Model.View()

	if !t.showSuggestions || len(t.suggestions) == 0 {
		if t.slashMode {
			return inputView + strings.Repeat("\n", maxVisibleSuggestions)
		}
		return inputView
	}

	// Render suggestions below input
	var sb strings.Builder
	sb.WriteString(inputView)
	sb.WriteString("\n")

	start := t.suggestionOffset
	if start < 0 {
		start = 0
	}
	end := start + maxVisibleSuggestions
	if end > len(t.suggestions) {
		end = len(t.suggestions)
	}

	for i := start; i < end; i++ {
		sug := t.suggestions[i]

		// Get command description
		cmd, ok := t.slashRegistry.Get(sug)
		if !ok {
			continue
		}

		if i == t.selectedIdx {
			sb.WriteString("    ")
			sb.WriteString(sugSelCmdStyle.Render(sug))
			sb.WriteString("  ")
			sb.WriteString(sugSelDescStyle.Render(cmd.Description))
		} else {
			sb.WriteString("    ")
			sb.WriteString(sugCmdStyle.Render(sug))
			sb.WriteString("  ")
			sb.WriteString(sugDescStyle.Render(cmd.Description))
		}

		sb.WriteString("\n")
	}
	// Pad remaining rows to fill the fixed slash suggestion area.
	rendered := end - start
	for i := rendered; i < maxVisibleSuggestions; i++ {
		sb.WriteString("\n")
	}

	return sb.String()
}

// Height returns the total height including suggestions area.
func (t TextInput) Height() int {
	rows := t.visibleInputRows()
	if t.slashMode {
		return rows + maxVisibleSuggestions
	}
	return rows
}

// IsSlashMode returns true if showing slash suggestions.
func (t TextInput) IsSlashMode() bool {
	return t.showSuggestions
}

// ClearSlashMode exits the slash suggestion reserved area.
func (t TextInput) ClearSlashMode() TextInput {
	t.slashMode = false
	t.showSuggestions = false
	t.slashDismissed = false
	t.suggestions = nil
	t.suggestionOffset = 0
	return t
}

// HasSuggestions returns true if there are visible suggestion candidates.
func (t TextInput) HasSuggestions() bool {
	return t.showSuggestions && len(t.suggestions) > 0
}

// SelectedSuggestionIndex exposes the currently highlighted suggestion for tests.
func (t TextInput) SelectedSuggestionIndex() int {
	return t.selectedIdx
}

// SuggestionOffset exposes the current suggestion window offset for tests.
func (t TextInput) SuggestionOffset() int {
	return t.suggestionOffset
}

// SelectedSuggestion returns the currently highlighted slash command.
func (t TextInput) SelectedSuggestion() string {
	if t.selectedIdx < 0 || t.selectedIdx >= len(t.suggestions) {
		return ""
	}
	return t.suggestions[t.selectedIdx]
}

func (t *TextInput) syncSuggestionWindow() {
	if len(t.suggestions) == 0 {
		t.suggestionOffset = 0
		return
	}

	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}
	if t.selectedIdx >= len(t.suggestions) {
		t.selectedIdx = len(t.suggestions) - 1
	}

	if t.selectedIdx < t.suggestionOffset {
		t.suggestionOffset = t.selectedIdx
	}
	if t.selectedIdx >= t.suggestionOffset+maxVisibleSuggestions {
		t.suggestionOffset = t.selectedIdx - maxVisibleSuggestions + 1
	}

	maxOffset := len(t.suggestions) - maxVisibleSuggestions
	if maxOffset < 0 {
		maxOffset = 0
	}
	if t.suggestionOffset > maxOffset {
		t.suggestionOffset = maxOffset
	}
	if t.suggestionOffset < 0 {
		t.suggestionOffset = 0
	}
}

func (t *TextInput) syncLayout() {
	t.Model.SetHeight(t.visibleInputRows())
}

func (t TextInput) visibleInputRows() int {
	width := t.Model.Width()
	if width < 1 {
		return 1
	}

	rows := 0
	for _, line := range strings.Split(t.Model.Value(), "\n") {
		lineWidth := rw.StringWidth(line)
		if lineWidth <= 0 {
			rows++
			continue
		}
		rows += (lineWidth-1)/width + 1
	}
	if rows < 1 {
		rows = 1
	}
	if rows > maxVisibleInputRows {
		rows = maxVisibleInputRows
	}
	return rows
}

func (t TextInput) cursorColumn() int {
	info := t.Model.LineInfo()
	return info.StartColumn + info.ColumnOffset
}

func splitSlashCommandAndSuffix(val string) (string, string) {
	for i, r := range val {
		if unicode.IsSpace(r) {
			return val[:i], val[i:]
		}
	}
	return val, ""
}

func keyEditsInput(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlH, tea.KeyCtrlW, tea.KeyCtrlU, tea.KeyCtrlK, tea.KeyEnter, tea.KeyCtrlV, tea.KeyInsert:
		return true
	default:
		return false
	}
}
