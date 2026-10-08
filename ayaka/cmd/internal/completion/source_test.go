package completion

import (
	"reflect"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/source"
)

func TestCompleteSrcRepoNamesLoadsRuntime(t *testing.T) {
	loads := 0
	sources := sourcerepos.New(func() ([]*source.SourceRepo, error) {
		loads++
		return []*source.SourceRepo{
			{Config: &source.SrcConfig{Name: "core"}},
			{Config: &source.SrcConfig{Name: "extra"}},
		}, nil
	})
	candidates, _ := CompleteSrcRepoNames(sources)(nil, nil, "")
	if !reflect.DeepEqual(candidates, []string{"core", "extra"}) {
		t.Fatalf("candidates = %v", candidates)
	}
	if loads != 1 {
		t.Fatalf("loader calls = %d, want 1", loads)
	}
}
