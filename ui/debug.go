package ui

import (
	"log"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

var debugKeyLogging atomic.Bool

// EnableDebugKeyLogging toggles low-level input logging for terminal event diagnosis.
func EnableDebugKeyLogging(enabled bool) {
	debugKeyLogging.Store(enabled)
}

func debugLogKey(prefix string, msg tea.KeyMsg) {
	if !debugKeyLogging.Load() {
		return
	}
	log.Printf("%s type=%v string=%q paste=%t alt=%t runes=%q", prefix, msg.Type, msg.String(), msg.Paste, msg.Alt, string(msg.Runes))
}

func debugLogMsg(prefix string, msg tea.Msg) {
	if !debugKeyLogging.Load() {
		return
	}
	log.Printf("%s type=%T", prefix, msg)
}
