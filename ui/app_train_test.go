package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/ui/components"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestTrainFixActionClearsStaleButtonAndKeepsCompletionMessage(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.handleEvent(model.Event{
		Type: model.TrainModeOpen,
		Train: &model.TrainEventData{
			RunID:    "primary",
			RawInput: "qwen3 lora",
			Model:    "qwen3",
			Method:   "lora",
		},
	})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type:    model.TrainAnalysisReady,
		Message: "Analysis complete. Ready to apply fix.",
		Train: &model.TrainEventData{
			RunID:       "primary",
			IssueType:   "failure",
			ActionID:    "fix-dsa-op",
			ActionKind:  "apply_patch",
			ActionLabel: "apply fix",
		},
	})
	app = next.(App)

	if got := len(app.trainView.GlobalActions.Items); got == 0 || app.trainView.GlobalActions.Items[0].ID != "fix-dsa-op" {
		t.Fatalf("expected fix action before execution, got %#v", app.trainView.GlobalActions.Items)
	}

	next, _ = app.handleEvent(model.Event{
		Type:    model.TrainActionApplied,
		Message: "op-agent: implementing DSA operator and compiling custom torch-npu...",
		Train: &model.TrainEventData{
			RunID:     "primary",
			IssueType: "failure",
			ActionID:  "fix-dsa-op",
		},
	})
	app = next.(App)

	run := app.trainView.RunByID("primary")
	if run == nil {
		t.Fatal("expected primary run")
	}
	if run.Phase != model.TrainPhaseFixing {
		t.Fatalf("expected fixing phase, got %s", run.Phase)
	}
	if len(run.AgentActions) != 0 {
		t.Fatalf("expected stale agent actions to be cleared, got %#v", run.AgentActions)
	}
	if got := len(app.trainView.GlobalActions.Items); got != 1 || app.trainView.GlobalActions.Items[0].ID != "stop" {
		t.Fatalf("expected only stop action while fixing, got %#v", app.trainView.GlobalActions.Items)
	}

	next, _ = app.handleEvent(model.Event{
		Type:    model.TrainFixApplied,
		Message: "op-agent: DSA operator finished. New torch wheel is ready. Please rerun experiment.",
		Train: &model.TrainEventData{
			RunID:      "primary",
			FixSummary: "DSA operator implemented and torch-npu recompiled",
		},
	})
	app = next.(App)

	run = app.trainView.RunByID("primary")
	if run == nil {
		t.Fatal("expected primary run after fix")
	}
	if run.Phase != model.TrainPhaseReady {
		t.Fatalf("expected ready phase after fix, got %s", run.Phase)
	}
	if !run.FixApplied {
		t.Fatal("expected run to be marked as fix-applied")
	}
	if got := len(app.trainView.GlobalActions.Items); got != 1 || app.trainView.GlobalActions.Items[0].Label != "rerun" {
		t.Fatalf("expected rerun action after fix, got %#v", app.trainView.GlobalActions.Items)
	}
	if run.StatusMessage != "op-agent: DSA operator finished. New torch wheel is ready. Please rerun experiment." {
		t.Fatalf("expected status message to keep fix completion text, got %q", run.StatusMessage)
	}
	last := app.state.Messages[len(app.state.Messages)-1]
	if !strings.Contains(last.Content, "DSA operator finished") {
		t.Fatalf("expected final agent message to include fix completion text, got %#v", last)
	}
}

func TestTrainViewUsesSharedChatSurface(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.TrainModeOpen,
		Train: &model.TrainEventData{
			RunID:    "primary",
			RawInput: "qwen3 lora",
			Model:    "qwen3",
			Method:   "lora",
		},
	})
	app = next.(App)

	view := app.View()
	if !strings.Contains(view, "train job") {
		t.Fatalf("expected train HUD in view, got:\n%s", view)
	}
	if strings.Contains(view, "setup env") {
		t.Fatalf("expected old train panel layout to be gone, got:\n%s", view)
	}
	if !strings.Contains(view, app.input.Model.Prompt) {
		t.Fatalf("expected global composer to stay visible, got:\n%s", view)
	}
}

func TestTrainHUDShowsSingleVIPLine(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 140, Height: 32})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.TrainModeOpen,
		Train: &model.TrainEventData{
			RunID:    "primary",
			RawInput: "qwen3 lora",
			Model:    "qwen3",
			Method:   "lora",
		},
	})
	app = next.(App)

	app.trainView.Request.Dataset = "alpaca_gpt4_zh"
	app.trainView.SetupContext.BaseModelRef = "models/qwen3-7b"
	app.trainView.Request.TargetName = "torch-npu-910b-0"
	if run := app.trainView.RunByID("primary"); run != nil {
		run.TargetName = "torch-npu-910b-0"
		run.Device = "Ascend"
	}
	app.trainView.UpsertCheck("primary", model.ChecklistItem{
		Group:   model.TrainCheckGroupLocal,
		Name:    "local_repo",
		Status:  model.TrainCheckPass,
		Summary: "repo detected",
	})
	app.trainView.UpsertCheck("primary", model.ChecklistItem{
		Group:   model.TrainCheckGroupTarget,
		Name:    "ssh",
		Status:  model.TrainCheckRunning,
		Summary: "checking target ssh",
	})

	view := app.View()
	if !strings.Contains(view, "run_id") || !strings.Contains(view, "machine") || !strings.Contains(view, "model") || !strings.Contains(view, "ckpt") || !strings.Contains(view, "dataset") {
		t.Fatalf("expected train HUD VIP fields, got:\n%s", view)
	}
	for _, want := range []string{"primary", "torch-npu-910b-0 npu", "qwen3", "qwen3-7b", "alpaca_gpt4_zh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected train HUD to contain %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "local checks") || strings.Contains(view, "target checks") {
		t.Fatalf("expected checklist details to stay out of train HUD, got:\n%s", view)
	}
}

func TestTrainSetupStreamsProgressAndSummaryToChat(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 140, Height: 32})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.TrainModeOpen,
		Train: &model.TrainEventData{
			RunID:    "primary",
			RawInput: "qwen3 lora alpaca_gpt4_zh",
			Model:    "qwen3",
			Method:   "lora",
		},
	})
	app = next.(App)

	events := []model.Event{
		{
			Type: model.TrainConnect,
			Train: &model.TrainEventData{
				RunID:   "primary",
				Host:    "torch-npu-910b-0",
				Address: "8.9.72.194:22",
				Status:  "connecting",
			},
		},
		{
			Type: model.TrainConnect,
			Train: &model.TrainEventData{
				RunID:   "primary",
				Host:    "torch-npu-910b-0",
				Address: "8.9.72.194:22",
				Status:  "connected",
			},
		},
		{
			Type: model.TrainSetup,
			Train: &model.TrainEventData{
				RunID:  "primary",
				Check:  "local_repo",
				Status: "checking",
				Scope:  "local",
			},
		},
		{
			Type: model.TrainSetup,
			Train: &model.TrainEventData{
				RunID:  "primary",
				Check:  "local_repo",
				Status: "passed",
				Detail: "repo detected",
				Scope:  "local",
			},
		},
		{
			Type: model.TrainSetup,
			Train: &model.TrainEventData{
				RunID:  "primary",
				Check:  "ssh",
				Status: "checking",
				Scope:  "target",
			},
		},
		{
			Type: model.TrainSetup,
			Train: &model.TrainEventData{
				RunID:  "primary",
				Check:  "ssh",
				Status: "passed",
				Detail: "weizheng@8.9.72.194:22",
				Scope:  "target",
			},
		},
		{
			Type:    model.TrainReady,
			Message: "all preflight checks passed. ready to start training.",
			Train: &model.TrainEventData{
				RunID:        "primary",
				ActionSource: "setup-helper",
			},
		},
	}

	for _, ev := range events {
		next, _ = app.handleEvent(ev)
		app = next.(App)
	}

	var contents []string
	for _, msg := range app.state.Messages {
		contents = append(contents, msg.Content)
	}
	all := strings.Join(contents, "\n")

	for _, want := range []string{
		"setup-agent",
		"connecting to torch-npu-910b-0 (8.9.72.194:22)...",
		"connected to torch-npu-910b-0 (8.9.72.194:22)",
		"checking repo...",
		"repo ok: repo detected",
		"checking ssh...",
		"ssh ok: weizheng@8.9.72.194:22",
		"setup summary",
		"local checks",
		"target checks",
		"[x] repo: repo detected",
		"[x] ssh: weizheng@8.9.72.194:22",
		"all preflight checks passed. ready to start training.",
		"╭",
		"╰",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("expected streamed setup content %q, got:\n%s", want, all)
		}
	}
}

func TestUpOnSingleLineMovesCursorToStart(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input.Model.SetValue("hello")
	app.input.Model.SetCursor(len("hello"))

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	app = next.(App)

	if got := app.input.Value(); got != "Xhello" {
		t.Fatalf("expected up to move cursor to line start, got %q", got)
	}
}

func TestDownOnSingleLineMovesCursorToEnd(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input.Model.SetValue("hello")
	app.input.Model.SetCursor(0)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	app = next.(App)

	if got := app.input.Value(); got != "helloY" {
		t.Fatalf("expected down to move cursor to line end, got %q", got)
	}
}

func TestUpAtSingleLineStartRecallsPreviousHistoryEntry(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("first prompt")
	app.input = app.input.PushHistory("second prompt")
	app.input.Model.SetValue("draft")
	app.input.Model.SetCursor(len("draft"))

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	if got := app.input.Value(); got != "draft" {
		t.Fatalf("expected first up to only move to line start, got %q", got)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	if got := app.input.Value(); got != "second prompt" {
		t.Fatalf("expected second up at line start to recall latest history entry, got %q", got)
	}
}

func TestDownAtSingleLineEndMovesForwardInHistory(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("single prompt")
	app.input.Model.SetValue("draft")
	app.input.Model.SetCursor(0)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if got := app.input.Value(); got != "single prompt" {
		t.Fatalf("expected first down on recalled single-line history to stay on the entry, got %q", got)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if got := app.input.Value(); got != "draft" {
		t.Fatalf("expected second down at recalled-line end to restore draft, got %q", got)
	}
}

func TestUpDownMoveCursorAcrossMultilineInput(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input.Model.SetValue("first line\nsecond line")
	app.input.Model.SetCursor(len("second line"))

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	if got := app.input.Model.Line(); got != 0 {
		t.Fatalf("expected up to move cursor to first logical line, got line %d", got)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if got := app.input.Model.Line(); got != 1 {
		t.Fatalf("expected down to move cursor back to second logical line, got line %d", got)
	}
}

func TestUpAtTopMultilineBoundaryMovesToLineStartThenHistory(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("older prompt")
	app.input.Model.SetValue("hell\nabbb\ncccc")

	for i := 0; i < 2; i++ {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
		app = next.(App)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	app = next.(App)
	if got := app.input.Value(); got != "Xhell\nabbb\ncccc" {
		t.Fatalf("expected third up at top boundary to move to line start, got %q", got)
	}

	app.input.Model.SetValue("hell\nabbb\ncccc")
	for i := 0; i < 2; i++ {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
		app = next.(App)
	}
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	if got := app.input.Value(); got != "older prompt" {
		t.Fatalf("expected fourth up at top line start to recall history, got %q", got)
	}
}

func TestUpAtTopMultilineBoundaryFromMiddleColumnMovesToLineStartWithoutWrapping(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input.Model.SetValue("hell\nabbb\ncccc")

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	app = next.(App)

	if got := app.input.Value(); got != "Xhell\nabbb\ncccc" {
		t.Fatalf("expected repeated up at top boundary from middle column to move to line start, got %q", got)
	}
}

func TestDownAtBottomMultilineBoundaryMovesToLineEndThenDraft(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("older prompt")
	app.input.Model.SetValue("hell\nabbb\ncccc")

	for i := 0; i < 3; i++ {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
		app = next.(App)
	}
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if got := app.input.Value(); got != "older prompt" {
		t.Fatalf("expected first down on recalled history to stay on the entry until line end, got %q", got)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if got := app.input.Value(); got != "hell\nabbb\ncccc" {
		t.Fatalf("expected second down from recalled history to restore draft, got %q", got)
	}
}

func TestDownAtBottomMultilineBoundaryFromMiddleColumnMovesToLineEndBeforeHistory(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("older prompt")
	app.input.Model.SetValue("hell\nabbb\ncccc")

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	app = next.(App)

	if got := app.input.Value(); got != "hell\nabbb\nccccX" {
		t.Fatalf("expected first down at bottom boundary from middle column to move to line end, got %q", got)
	}
}

func TestHistoryRecallPlacesCursorAtFirstLineStartForRepeatedUp(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("first command")
	app.input = app.input.PushHistory("second command")
	app.input.Model.SetValue("draft")
	app.input.Model.SetCursor(0)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	if got := app.input.Value(); got != "second command" {
		t.Fatalf("expected first up at draft start to recall latest history, got %q", got)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	if got := app.input.Value(); got != "first command" {
		t.Fatalf("expected second consecutive up to recall previous history, got %q", got)
	}
}

func TestDownMovesToNextHistoryEntryWithCursorAtLastLineEnd(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("older line\nolder tail")
	app.input = app.input.PushHistory("newer line\nnewer tail")
	app.input.Model.SetValue("")
	app.input.Model.SetCursor(0)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	app = next.(App)

	if got := app.input.Value(); got != "newer line\nnewer tailX" {
		t.Fatalf("expected down-recalled history cursor at last-line end, got %q", got)
	}
}

func TestUpAtOldestHistoryEntryKeepsCursorAtFirstLineStart(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("older line\nsecond line")
	app.input.Model.SetValue("")
	app.input.Model.SetCursor(0)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	app = next.(App)

	if got := app.input.Value(); got != "Xolder line\nsecond line" {
		t.Fatalf("expected extra up at oldest history to stay at first-line start, got %q", got)
	}
}

func TestBackslashEnterInsertsNewlineWithoutSubmitting(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input.Model.SetValue("first\\")
	app.input.Model.SetCursor(len("first\\"))

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	if got := app.input.Value(); got != "first\n" {
		t.Fatalf("expected backslash-enter to insert newline, got %q", got)
	}
	if len(app.state.Messages) != 0 {
		t.Fatalf("expected no chat submit on backslash-enter, got %#v", app.state.Messages)
	}
	select {
	case msg := <-userCh:
		t.Fatalf("expected no backend submit on backslash-enter, got %q", msg)
	default:
	}
}

func TestBackslashEnterKeepsPreviousLineVisibleWithoutRepeatingPrompt(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 18, Height: 20})
	app = next.(App)

	for _, r := range "first\\" {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = next.(App)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	view := app.input.View()
	if !strings.Contains(view, "first") {
		t.Fatalf("expected first line to remain visible after continuation newline, got %q", view)
	}
	if got := strings.Count(view, "❯ "); got != 1 {
		t.Fatalf("expected prompt to render once in multiline composer, got %d in %q", got, view)
	}
}

func TestSlashCommandEnterSubmitsInsteadOfAcceptingSuggestion(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	for _, r := range "/ex" {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = next.(App)
	}
	if !app.input.IsSlashMode() {
		t.Fatal("expected slash mode to be active before submitting slash command")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	if got := app.input.Value(); got != "" {
		t.Fatalf("expected composer to reset after slash command submit, got %q", got)
	}
	if app.input.IsSlashMode() {
		t.Fatal("expected slash mode to close after slash command submit")
	}
	select {
	case msg := <-userCh:
		if msg != "/exit" {
			t.Fatalf("expected slash command submit to autocomplete and forward /exit, got %q", msg)
		}
	default:
		t.Fatal("expected slash command submit to reach backend channel")
	}
}

func TestSlashCommandTabClosesSuggestionList(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	for _, r := range "/ex" {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = next.(App)
	}
	if !app.input.HasSuggestions() {
		t.Fatal("expected slash suggestions before tab completion")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	app = next.(App)
	if got := app.input.Value(); got != "/exit " {
		t.Fatalf("expected tab to accept the selected slash command, got %q", got)
	}
	if app.input.IsSlashMode() || app.input.HasSuggestions() {
		t.Fatal("expected slash suggestions to close after tab completion")
	}

	next, _ = app.Update(cursor.BlinkMsg{})
	app = next.(App)
	if app.input.IsSlashMode() || app.input.HasSuggestions() {
		t.Fatal("expected slash suggestions to stay closed after tab completion on follow-up updates")
	}
}

func TestSlashCommandEscClosesSuggestionListUntilNextEdit(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	for _, r := range "/ex" {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = next.(App)
	}
	if !app.input.HasSuggestions() {
		t.Fatal("expected slash suggestions before esc dismissal")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	app = next.(App)
	if app.input.IsSlashMode() || app.input.HasSuggestions() {
		t.Fatal("expected esc to close slash suggestions")
	}

	next, _ = app.Update(cursor.BlinkMsg{})
	app = next.(App)
	if app.input.IsSlashMode() || app.input.HasSuggestions() {
		t.Fatal("expected slash suggestions to stay closed after esc on follow-up updates")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	app = next.(App)
	if !app.input.IsSlashMode() || !app.input.HasSuggestions() {
		t.Fatal("expected editing after esc dismissal to reopen slash suggestions")
	}
}

func TestEnterSubmitsWhenBackslashIsNotImmediatePreviousCharacter(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input.Model.SetValue("first\\ ")
	app.input.Model.SetCursor(len("first\\ "))

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	if got := len(app.state.Messages); got != 1 {
		t.Fatalf("expected one submitted user message, got %d", got)
	}
	if got := app.state.Messages[0].Content; got != "first\\" {
		t.Fatalf("expected submitted content to keep literal backslash, got %q", got)
	}
	select {
	case msg := <-userCh:
		if msg != "first\\" {
			t.Fatalf("expected backend submit to keep literal backslash, got %q", msg)
		}
	default:
		t.Fatal("expected backend submit on normal enter")
	}
}

func TestSubmittingMultilinePromptPreservesFormatting(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input.Model.SetValue("line one\nline two")
	app.input.Model.SetCursor(len("line two"))

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	if got := len(app.state.Messages); got != 1 {
		t.Fatalf("expected one submitted user message, got %d", got)
	}
	if got := app.state.Messages[0].Content; got != "line one\nline two" {
		t.Fatalf("expected submitted content to preserve multiline formatting, got %q", got)
	}
	select {
	case msg := <-userCh:
		if msg != "line one\nline two" {
			t.Fatalf("expected backend submit to preserve multiline formatting, got %q", msg)
		}
	default:
		t.Fatal("expected backend submit for multiline prompt")
	}
}

func TestCtrlVPastesMultilinePromptIntoComposerWithoutSubmitting(t *testing.T) {
	if err := clipboard.WriteAll("line one\nline two"); err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}

	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	next, cmd := app.handleKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	app = next.(App)
	if cmd == nil {
		t.Fatal("expected ctrl+v to return a paste command")
	}

	msg := cmd()
	next, _ = app.Update(msg)
	app = next.(App)

	if got := app.input.Value(); got != "line one\nline two" {
		t.Fatalf("expected ctrl+v paste to preserve multiline content, got %q", got)
	}
	if len(app.state.Messages) != 0 {
		t.Fatalf("expected pasted content to stay in composer, got %#v", app.state.Messages)
	}
	select {
	case msg := <-userCh:
		t.Fatalf("expected no backend submit while pasting, got %q", msg)
	default:
	}
}

func TestInsertPastesMultilinePromptIntoComposerWithoutSubmitting(t *testing.T) {
	if err := clipboard.WriteAll("line one\nline two"); err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}

	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	next, cmd := app.handleKey(tea.KeyMsg{Type: tea.KeyInsert})
	app = next.(App)
	if cmd != nil {
		t.Fatal("expected insert paste to happen synchronously without a follow-up command")
	}

	if got := app.input.Value(); got != "line one\nline two" {
		t.Fatalf("expected insert paste to preserve multiline content, got %q", got)
	}
	if len(app.state.Messages) != 0 {
		t.Fatalf("expected pasted content to stay in composer, got %#v", app.state.Messages)
	}
	select {
	case msg := <-userCh:
		t.Fatalf("expected no backend submit while insert pasting, got %q", msg)
	default:
	}
}

func TestInsertPasteSuppressesFollowingNativePasteKeyStream(t *testing.T) {
	if err := clipboard.WriteAll("line one\nline two"); err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}

	userCh := make(chan string, 8)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyInsert})
	app = next.(App)

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("line one")},
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune("line two")},
	} {
		next, _ = app.handleKey(msg)
		app = next.(App)
	}

	if got := app.input.Value(); got != "line one\nline two" {
		t.Fatalf("expected duplicate native paste stream to be suppressed, got %q", got)
	}
	if len(app.state.Messages) != 0 {
		t.Fatalf("expected suppressed native paste stream not to submit messages, got %#v", app.state.Messages)
	}
	select {
	case msg := <-userCh:
		t.Fatalf("expected no backend submit while suppressing duplicate paste stream, got %q", msg)
	default:
	}
}

func TestBracketedPasteMultilinePromptIntoComposerWithoutSubmitting(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	next, _ = app.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("line one\nline two"),
		Paste: true,
	})
	app = next.(App)

	if got := app.input.Value(); got != "line one\nline two" {
		t.Fatalf("expected bracketed paste to preserve multiline content, got %q", got)
	}
	if len(app.state.Messages) != 0 {
		t.Fatalf("expected pasted content to stay in composer, got %#v", app.state.Messages)
	}
	select {
	case msg := <-userCh:
		t.Fatalf("expected no backend submit while bracketed pasting, got %q", msg)
	default:
	}
}

func TestPasteKeyStreamWithNewlineEnterDoesNotSubmitPerLine(t *testing.T) {
	userCh := make(chan string, 4)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("line one"), Paste: true},
		{Type: tea.KeyEnter, Paste: true},
		{Type: tea.KeyRunes, Runes: []rune("line two"), Paste: true},
		{Type: tea.KeyEnter, Paste: true},
		{Type: tea.KeyRunes, Runes: []rune("line three"), Paste: true},
	} {
		next, _ = app.handleKey(msg)
		app = next.(App)
	}

	if got := app.input.Value(); got != "line one\nline two\nline three" {
		t.Fatalf("expected pasted key stream to stay in composer, got %q", got)
	}
	if len(app.state.Messages) != 0 {
		t.Fatalf("expected pasted key stream not to submit messages, got %#v", app.state.Messages)
	}
	select {
	case msg := <-userCh:
		t.Fatalf("expected pasted key stream not to reach backend, got %q", msg)
	default:
	}
}

func TestSlashSuggestionsDoNotMoveOnThinkingTick(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	for _, r := range "/train" {
		next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = next.(App)
	}
	if !app.input.HasSuggestions() {
		t.Fatal("expected slash suggestions before background tick")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	selectedBefore := app.input.SelectedSuggestionIndex()
	offsetBefore := app.input.SuggestionOffset()
	viewBefore := app.input.View()

	next, _ = app.Update(components.TickMsg{Time: time.Now()})
	app = next.(App)

	if got := app.input.SelectedSuggestionIndex(); got != selectedBefore {
		t.Fatalf("expected thinking tick to keep selected suggestion index %d, got %d", selectedBefore, got)
	}
	if got := app.input.SuggestionOffset(); got != offsetBefore {
		t.Fatalf("expected thinking tick to keep suggestion offset %d, got %d", offsetBefore, got)
	}
	if got := app.input.View(); got != viewBefore {
		t.Fatalf("expected thinking tick to leave suggestion view unchanged\nbefore:\n%q\nafter:\n%q", viewBefore, got)
	}
}

func TestHistoryRecallOfSlashCommandDoesNotReopenSlashSuggestionsOnBlink(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	app = next.(App)

	app.input = app.input.PushHistory("/compact")
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	app = next.(App)
	if got := app.input.Value(); got != "/compact" {
		t.Fatalf("expected slash history recall, got %q", got)
	}
	if app.input.IsSlashMode() {
		t.Fatal("expected slash suggestions to stay closed immediately after slash history recall")
	}

	next, _ = app.Update(cursor.BlinkMsg{})
	app = next.(App)
	if app.input.IsSlashMode() {
		t.Fatal("expected slash suggestions to stay closed while browsing slash history")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if got := app.input.Value(); got != "/compact" {
		t.Fatalf("expected first down on recalled slash history to stay on the entry, got %q", got)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if got := app.input.Value(); got != "" {
		t.Fatalf("expected second down to leave slash history recall and restore draft, got %q", got)
	}
}

func TestAppViewKeepsComposerContinuationIndented(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = next.(App)

	app.input.Model.SetValue("hello\nworld\n!!!")
	app.input.Model.SetCursor(len("!!!"))

	view := app.View()
	for _, want := range []string{"  ❯ hello", "\n    world", "\n    !!!"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected multiline composer indentation %q in view, got:\n%s", want, view)
		}
	}
}

func TestSubmittingMultilinePromptRendersAlignedUserMessageBlock(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = next.(App)

	app.input.Model.SetValue("hello\nworld\n!!!")
	app.input.Model.SetCursor(len("!!!"))

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	view := app.View()
	for _, want := range []string{"  > hello", "\n    world", "\n    !!!"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected multiline submitted message indentation %q in view, got:\n%s", want, view)
		}
	}
}

func TestEscInterruptTokenSentForQueuedTrain(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false
	app.trainView.Active = true
	app.queuedInputs = []string{"/train qwen3 lora"}
	app.trainView.Runs = []model.TrainRunState{{
		ID:    "primary",
		Phase: model.TrainPhaseSetup,
	}}
	app.trainView.ActiveRunID = "primary"

	next, _ := app.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	app = next.(App)
	_ = app

	select {
	case msg := <-userCh:
		if msg != "/train exit" {
			t.Fatalf("expected esc to send /train exit for queued interrupt, got %q", msg)
		}
	default:
		t.Fatal("expected esc to send /train exit for queued interrupt")
	}
}

func TestBusyTrainQueuesInputInBannerInsteadOfChatStream(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	app = next.(App)
	app.trainView.Active = true
	app.trainView.Runs = []model.TrainRunState{{
		ID:    "primary",
		Phase: model.TrainPhaseSetup,
	}}
	app.trainView.ActiveRunID = "primary"

	app.input.Model.SetValue("/train qwen3 lora")
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	if got := len(app.queuedInputs); got != 1 {
		t.Fatalf("expected one queued input, got %d", got)
	}
	if len(app.state.Messages) != 0 {
		t.Fatalf("expected queued input to stay out of chat stream, got %#v", app.state.Messages)
	}
	select {
	case msg := <-userCh:
		t.Fatalf("expected no immediate backend submit while busy, got %q", msg)
	default:
	}

	view := app.View()
	if !strings.Contains(view, "messages queued (press esc to interrupt)") {
		t.Fatalf("expected queued-input banner in view, got:\n%s", view)
	}
	if strings.Contains(view, "> /train qwen3 lora") {
		t.Fatalf("expected queued command to stay out of chat stream, got:\n%s", view)
	}
}
