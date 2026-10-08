package sourcerepos

import (
	"errors"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/source"
)

func TestLazyReaderCachesInputWithoutExposingItsSlice(t *testing.T) {
	calls := 0
	repository := &source.SourceRepo{Config: &source.SrcConfig{Name: "test"}}
	reader := New(func() ([]*source.SourceRepo, error) { calls++; return []*source.SourceRepo{repository}, nil })
	if calls != 0 {
		t.Fatal("configuration loaded during construction")
	}
	list, err := reader.All()
	if err != nil {
		t.Fatal(err)
	}
	list[0] = nil
	found, err := reader.Find("test")
	if err != nil || found != repository || calls != 1 {
		t.Fatalf("found=%v error=%v calls=%d", found, err, calls)
	}
}

func TestLazyReaderCachesFailures(t *testing.T) {
	want := errors.New("invalid configuration")
	calls := 0
	reader := New(func() ([]*source.SourceRepo, error) { calls++; return nil, want })
	for range 2 {
		if _, err := reader.Find("test"); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}
