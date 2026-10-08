package keyinput

import (
	"errors"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
)

func TestLoadRetriesWithInteractivePassphrase(t *testing.T) {
	want := &sign.SigningKey{}
	var passes []string
	provider := Provider{
		Secret: func(string, string, func() (string, error)) (string, error) { return "", nil },
		Prompt: func(string) (string, error) { return "unlock", nil },
		LoadKey: func(dir, pass string) (*sign.SigningKey, error) {
			if dir != "/keys" {
				t.Fatalf("key directory = %q", dir)
			}
			passes = append(passes, pass)
			if pass == "unlock" {
				return want, nil
			}
			return nil, errors.New("encrypted key")
		},
	}
	key, pass, err := provider.Load(Options{Home: "/keys"})
	if err != nil || key != want || pass != "unlock" || len(passes) != 2 {
		t.Fatalf("key=%v pass=%q attempts=%v error=%v", key, pass, passes, err)
	}
}

func TestLoadDoesNotPromptInNonInteractiveMode(t *testing.T) {
	want := errors.New("encrypted key")
	provider := Provider{
		Secret:  func(string, string, func() (string, error)) (string, error) { return "", nil },
		LoadKey: func(string, string) (*sign.SigningKey, error) { return nil, want },
	}
	if _, _, err := provider.Load(Options{Home: "/keys"}); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestPassphraseForwardsOnlyEnabledPrompt(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		provider := Provider{
			Prompt: func(string) (string, error) { return "prompted", nil },
			Secret: func(env, file string, prompt func() (string, error)) (string, error) {
				if env != PassphraseEnv || file != "secret.txt" {
					t.Fatalf("secret input env=%q file=%q", env, file)
				}
				if (prompt != nil) != enabled {
					t.Fatalf("prompt enabled = %v, want %v", prompt != nil, enabled)
				}
				return "", nil
			},
		}
		if _, err := provider.Passphrase(Options{PassphraseFile: "secret.txt"}, enabled); err != nil {
			t.Fatal(err)
		}
	}
}
