package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/client"
	"github.com/spf13/cobra"
)

func TestUploadOverrideDoesNotMutateSavedCredentials(t *testing.T) {
	endpoint := &AyatoServer{URL: "https://example.test", AccessToken: "saved"}
	var got string
	provider := Provider{
		BearerClient: func(_ string, token string) (*client.Client, error) { got = token; return nil, nil },
		Prompt:       func() (string, error) { t.Fatal("explicit token prompted for credentials"); return "", nil },
	}
	if _, err := provider.Client(endpoint, UploadOptions{Token: "explicit", Password: "legacy", AskPass: true}); err != nil {
		t.Fatal(err)
	}
	if got != "explicit" || endpoint.AccessToken != "saved" {
		t.Fatalf("override=%q saved=%q", got, endpoint.AccessToken)
	}
}

func TestRemovalKeepsCredentialResolutionLazyAndCachesErrors(t *testing.T) {
	command := &cobra.Command{}
	AddRepoServerFlags(command)
	calls := 0
	want := errors.New("no saved server")
	provider := Provider{Resolve: func(string) (*AyatoServer, error) { calls++; return nil, want }}
	remove, err := provider.Removal(command)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("preparing a dry run read credentials")
	}
	for range 2 {
		if err := remove(context.Background(), "test", "package"); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("credential resolutions = %d", calls)
	}
}
