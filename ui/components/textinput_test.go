package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/ui/slash"
)

func TestTextInputHistoryRecall(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("first prompt")
	input = input.PushHistory("second prompt")
	input.Model.SetValue("draft")
	input.Model.SetCursor(len("draft"))

	input = input.PrevHistory()
	if got := input.Value(); got != "second prompt" {
		t.Fatalf("expected latest history entry, got %q", got)
	}

	input = input.PrevHistory()
	if got := input.Value(); got != "first prompt" {
		t.Fatalf("expected previous history entry, got %q", got)
	}

	input = input.NextHistory()
	if got := input.Value(); got != "second prompt" {
		t.Fatalf("expected forward history entry, got %q", got)
	}

	input = input.NextHistory()
	if got := input.Value(); got != "draft" {
		t.Fatalf("expected draft restoration after leaving history, got %q", got)
	}
}

func TestTextInputPrevHistoryPlacesCursorAtFirstLineStart(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("older line\nsecond line")
	input.Model.SetValue("draft")
	input.Model.SetCursor(len("draft"))

	input = input.PrevHistory()
	input = input.InsertText("X")

	if got := input.Value(); got != "Xolder line\nsecond line" {
		t.Fatalf("expected recalled history cursor at first-line start, got %q", got)
	}
}

func TestTextInputPrevHistoryAtOldestEntryDoesNotJumpToEnd(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("older line\nsecond line")

	input = input.PrevHistory()
	input = input.PrevHistory()
	input = input.InsertText("X")

	if got := input.Value(); got != "Xolder line\nsecond line" {
		t.Fatalf("expected oldest-history up to keep cursor at first-line start, got %q", got)
	}
}

func TestTextInputNextHistoryPlacesCursorAtLastLineEnd(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("older line\nolder tail")
	input = input.PushHistory("newer line\nnewer tail")
	input.Model.SetValue("")
	input.Model.SetCursor(0)

	input = input.PrevHistory()
	input = input.PrevHistory()
	input = input.NextHistory()
	input = input.InsertText("X")

	if got := input.Value(); got != "newer line\nnewer tailX" {
		t.Fatalf("expected next-history recall cursor at last-line end, got %q", got)
	}
}

func TestTextInputHistoryDoesNotBreakSlashSuggestions(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("/project")
	var cmd tea.Cmd
	input, cmd = input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	_ = cmd
	if !input.IsSlashMode() {
		t.Fatal("expected slash suggestions after typing slash")
	}
}

func TestTextInputHistoryRecallOfSlashCommandDoesNotReopenSuggestions(t *testing.T) {
	input := NewTextInput()
	input = input.PushHistory("/project")
	input = input.PushHistory("hello")

	input = input.PrevHistory()
	if got := input.Value(); got != "hello" {
		t.Fatalf("expected latest history entry, got %q", got)
	}

	input = input.PrevHistory()
	if got := input.Value(); got != "/project" {
		t.Fatalf("expected slash command from history, got %q", got)
	}
	if input.IsSlashMode() {
		t.Fatal("expected slash suggestions to stay closed while browsing history")
	}

	input = input.NextHistory()
	if got := input.Value(); got != "hello" {
		t.Fatalf("expected down to continue history recall even in slash mode, got %q", got)
	}
}

func TestTextInputSuggestionsScrollDownToKeepSelectionVisible(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 7
	input.suggestionOffset = 0

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyDown})

	if input.selectedIdx != 8 {
		t.Fatalf("expected selected index 8, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 1 {
		t.Fatalf("expected suggestion offset 1, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd08") {
		t.Fatalf("expected view to include newly selected command, got %q", view)
	}
	if strings.Contains(view, "/cmd00") {
		t.Fatalf("expected first command to scroll out of view, got %q", view)
	}
}

func TestTextInputSuggestionsScrollUpToKeepSelectionVisible(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 2
	input.suggestionOffset = 2

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyUp})

	if input.selectedIdx != 1 {
		t.Fatalf("expected selected index 1, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 1 {
		t.Fatalf("expected suggestion offset 1, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd01") {
		t.Fatalf("expected view to include newly selected command, got %q", view)
	}
	if strings.Contains(view, "/cmd09") {
		t.Fatalf("expected last command to scroll out of view, got %q", view)
	}
}

func TestTextInputSuggestionsWrapDownToTopPage(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 9
	input.suggestionOffset = 2

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyDown})

	if input.selectedIdx != 0 {
		t.Fatalf("expected selected index 0 after wrap, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 0 {
		t.Fatalf("expected suggestion offset 0 after wrap, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd00") {
		t.Fatalf("expected wrapped view to include first command, got %q", view)
	}
	if strings.Contains(view, "/cmd09") {
		t.Fatalf("expected last command to leave view after wrapping to top, got %q", view)
	}
}

func TestTextInputSuggestionsWrapUpToLastPage(t *testing.T) {
	input := newSlashSuggestionInput(10)
	input.selectedIdx = 0
	input.suggestionOffset = 0

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyUp})

	if input.selectedIdx != 9 {
		t.Fatalf("expected selected index 9 after wrap, got %d", input.selectedIdx)
	}
	if input.suggestionOffset != 2 {
		t.Fatalf("expected suggestion offset 2 after wrap, got %d", input.suggestionOffset)
	}

	view := input.View()
	if !strings.Contains(view, "/cmd09") {
		t.Fatalf("expected wrapped view to include last command, got %q", view)
	}
	if strings.Contains(view, "/cmd00") {
		t.Fatalf("expected first command to leave view after wrapping to bottom, got %q", view)
	}
}

func TestTextInputSuggestionTabAcceptsSelection(t *testing.T) {
	input := newSlashSuggestionInput(3)
	input.Model.SetValue("/ex")
	input.selectedIdx = 1

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyTab})

	if got := input.Value(); got != "/cmd01 " {
		t.Fatalf("expected tab to accept selected suggestion, got %q", got)
	}
	if input.IsSlashMode() {
		t.Fatal("expected slash mode to close after tab acceptance")
	}
}

func TestTextInputSuggestionEnterDoesNotAcceptSelection(t *testing.T) {
	input := newSlashSuggestionInput(3)
	input.Model.SetValue("/ex")
	input.selectedIdx = 1

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := input.Value(); got != "/ex" {
		t.Fatalf("expected enter to leave slash input untouched, got %q", got)
	}
	if !input.IsSlashMode() {
		t.Fatal("expected slash mode to remain active after enter is ignored by the input component")
	}
}

func TestTextInputSuggestionTabClosesSuggestionList(t *testing.T) {
	input := newSlashSuggestionInput(3)
	input.Model.SetValue("/ex")
	input.selectedIdx = 1

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyTab})
	input, _ = input.Update(cursor.BlinkMsg{})

	if input.IsSlashMode() {
		t.Fatal("expected slash mode to stay closed after tab completion")
	}
	if input.HasSuggestions() {
		t.Fatal("expected suggestions to stay hidden after tab completion")
	}
}

func TestTextInputSuggestionEscClosesSuggestionListUntilNextEdit(t *testing.T) {
	input := newSlashSuggestionInput(3)
	input.Model.SetValue("/ex")

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyEsc})
	input, _ = input.Update(cursor.BlinkMsg{})

	if input.IsSlashMode() {
		t.Fatal("expected slash mode to stay closed after esc dismissal")
	}
	if input.HasSuggestions() {
		t.Fatal("expected suggestions to stay hidden after esc dismissal")
	}

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if !input.IsSlashMode() || !input.HasSuggestions() {
		t.Fatal("expected editing after esc dismissal to reopen slash suggestions")
	}
}

func TestTextInputResolvedSlashSubmitValueUsesSelectedSuggestion(t *testing.T) {
	input := newSlashSuggestionInput(3)
	input.suggestions = []string{"/claim", "/clear", "/close"}
	input.showSuggestions = true
	input.slashMode = true
	input.selectedIdx = 2
	input.Model.SetValue("/cl some-arg")

	if got := input.ResolvedSubmitValue(); got != "/close some-arg" {
		t.Fatalf("expected resolved slash submit value to use selected suggestion, got %q", got)
	}
}

func TestTextInputPreservesMultilineValue(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(40)
	input.Model.SetValue("first line\nsecond line")

	if got := input.Value(); got != "first line\nsecond line" {
		t.Fatalf("expected multiline value to be preserved, got %q", got)
	}
	if got := input.Height(); got != 2 {
		t.Fatalf("expected two visible input rows, got %d", got)
	}
}

func TestTextInputUsesOriginalPromptAndOnlyShowsItOnFirstComposerLine(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(40)
	input.Model.SetValue("first line\nsecond line")

	if got := input.Model.Prompt; got != "❯ " {
		t.Fatalf("expected original prompt, got %q", got)
	}

	view := input.View()
	if got := strings.Count(view, "❯ "); got != 1 {
		t.Fatalf("expected prompt to appear once in multiline composer, got %d in %q", got, view)
	}
}

func TestTextInputHeightCapsVisibleRows(t *testing.T) {
	input := NewTextInput()
	input = input.SetWidth(40)
	input.Model.SetValue(strings.Repeat("line\n", maxVisibleInputRows+2) + "tail")

	if got := input.Height(); got != maxVisibleInputRows {
		t.Fatalf("expected visible height cap %d, got %d", maxVisibleInputRows, got)
	}
}

func newSlashSuggestionInput(count int) TextInput {
	input := NewTextInput()
	registry := slash.NewRegistry()
	input.slashRegistry = registry
	input.showSuggestions = true
	input.slashMode = true
	input.suggestions = make([]string, 0, count)

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("/cmd%02d", i)
		registry.Register(slash.Command{
			Name:        name,
			Description: fmt.Sprintf("Command %02d", i),
			Usage:       name,
		})
		input.suggestions = append(input.suggestions, name)
	}

	return input
}
