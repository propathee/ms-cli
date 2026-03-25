package slash

import (
	"reflect"
	"testing"
)

func TestSuggestionsAreSortedDeterministically(t *testing.T) {
	r := NewRegistry()

	r.Register(Command{Name: "/zeta"})
	r.Register(Command{Name: "/alpha"})
	r.Register(Command{Name: "/gamma"})

	got := r.Suggestions("/")
	want := append([]string{}, got...)
	for i := 1; i < len(want); i++ {
		if want[i-1] > want[i] {
			t.Fatalf("expected slash suggestions to be sorted, got %v", got)
		}
	}

	gotAgain := r.Suggestions("/")
	if !reflect.DeepEqual(got, gotAgain) {
		t.Fatalf("expected repeated suggestion lookups to stay stable, first %v second %v", got, gotAgain)
	}
}
