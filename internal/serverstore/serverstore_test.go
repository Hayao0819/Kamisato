package serverstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type fakeKeyring struct {
	data        map[string]string
	unavailable bool
}

var errKeyringUnavailable = errors.New("keyring unavailable")

func newFakeKeyring() *fakeKeyring { return &fakeKeyring{data: make(map[string]string)} }

func (f *fakeKeyring) Get(service, key string) (string, error) {
	if f.unavailable {
		return "", errKeyringUnavailable
	}
	value, ok := f.data[service+":"+key]
	if !ok {
		return "", errKeyringNotFound
	}
	return value, nil
}

func (f *fakeKeyring) Set(service, key, secret string) error {
	if f.unavailable {
		return errKeyringUnavailable
	}
	f.data[service+":"+key] = secret
	return nil
}

func (f *fakeKeyring) Delete(service, key string) error {
	if f.unavailable {
		return errKeyringUnavailable
	}
	mapKey := service + ":" + key
	if _, ok := f.data[mapKey]; !ok {
		return errKeyringNotFound
	}
	delete(f.data, mapKey)
	return nil
}

func useKeyring(t *testing.T, backend keyring) {
	t.Helper()
	previous := secretKeyring
	secretKeyring = backend
	t.Cleanup(func() { secretKeyring = previous })
}

func TestKeyringIdentifiersAndFileFallbackRemainCompatible(t *testing.T) {
	fake := newFakeKeyring()
	useKeyring(t, fake)

	if stored, err := storeAccessToken("https://ayato.example", "access"); err != nil || !stored {
		t.Fatal("access token was not stored")
	}
	if stored, err := storeRefreshToken("https://ayato.example", "refresh"); err != nil || !stored {
		t.Fatal("refresh token was not stored")
	}
	if got := fake.data["kamisato-ayato:https://ayato.example"]; got != "access" {
		t.Fatalf("legacy access key = %q", got)
	}
	if got := fake.data["kamisato-ayato:https://ayato.example\x00refresh"]; got != "refresh" {
		t.Fatalf("legacy refresh key = %q", got)
	}

	fake.unavailable = true
	if got := loadAccessTokenValue("https://ayato.example", "file-access"); got != "file-access" {
		t.Fatalf("file fallback = %q", got)
	}
	if got := loadRefreshTokenValue("https://ayato.example"); got != "" {
		t.Fatalf("refresh token must remain keyring-only, got %q", got)
	}
}

func TestFailedCredentialDeletionCannotReactivateStaleSecrets(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)
	server := "https://ayato.example"
	if err := SaveEndpoint(server, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := storeAccessToken(server, "old-keyring-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := storeRefreshToken(server, "old-refresh"); err != nil {
		t.Fatal(err)
	}
	fake.unavailable = true
	if err := ClearCredentials(server, false); err != nil {
		t.Fatalf("explicit inactive marker must make keyring cleanup best-effort: %v", err)
	}
	if got := loadAccessTokenValue(server, "new-file-token"); got != "" {
		t.Fatalf("inactive marker allowed a stale credential: %q", got)
	}
	if got := loadRefreshTokenValue(server); got != "" {
		t.Fatalf("inactive marker allowed a stale refresh credential: %q", got)
	}
}

func TestStaticFileFallbackDisablesStaleRefreshWhenKeyringUnavailable(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)
	server := "https://headless.example"
	if _, err := storeRefreshToken(server, "old-refresh"); err != nil {
		t.Fatal(err)
	}
	fake.unavailable = true
	if err := saveCredentialMode(server, credentialMode{Access: credentialSourceFile, Refresh: credentialSourceNone}); err != nil {
		t.Fatal(err)
	}
	if got := loadAccessTokenValue(server, "new-static-token"); got != "new-static-token" {
		t.Fatalf("static file fallback = %q", got)
	}
	if got := loadRefreshTokenValue(server); got != "" {
		t.Fatalf("stale refresh token became active: %q", got)
	}
}

func TestTokenSourcesCoordinateRotationThroughCompatibleStore(t *testing.T) {
	dataDirectory := t.TempDir()
	cacheDirectory := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDirectory)
	t.Setenv("XDG_CACHE_HOME", cacheDirectory)
	fake := newFakeKeyring()
	useKeyring(t, fake)

	server := "https://ayato.example/prefix"
	db := blinkyutils.Registry{Default: server, Endpoints: map[string]blinkyutils.StoredEndpoint{
		server: {Username: "alice", AccessToken: "legacy-file-token"},
	}}
	if err := saveDB(db); err != nil {
		t.Fatal(err)
	}
	if err := SaveTokens(server, "alice", "old-access", "old-refresh"); err != nil {
		t.Fatal(err)
	}

	first := NewTokenSource(&Endpoint{URL: server, Username: "alice", AccessToken: "old-access"})
	second := NewTokenSource(&Endpoint{URL: server, Username: "alice", AccessToken: "old-access"})
	var refreshCalls atomic.Int32
	refresh := func(_ context.Context, gotServer, gotRefresh string) (client.TokenPair, error) {
		refreshCalls.Add(1)
		if gotServer != server || gotRefresh != "old-refresh" {
			return client.TokenPair{}, errors.NewErrf("refresh arguments = %q, %q", gotServer, gotRefresh)
		}
		return client.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh", Login: "alice"}, nil
	}
	first.refresh = refresh
	second.refresh = refresh

	var wait sync.WaitGroup
	errorsFromRefresh := make(chan error, 2)
	for _, source := range []*TokenSource{first, second} {
		wait.Add(1)
		go func(source *TokenSource) {
			defer wait.Done()
			errorsFromRefresh <- source.RefreshIfCurrent(context.Background(), "old-access")
		}(source)
	}
	wait.Wait()
	close(errorsFromRefresh)
	for err := range errorsFromRefresh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("refresh calls = %d, want exactly one", refreshCalls.Load())
	}
	if token, _ := second.Token(context.Background()); token != "new-access" {
		t.Fatalf("second source token = %q", token)
	}
	if got := loadRefreshTokenValue(server); got != "new-refresh" {
		t.Fatalf("rotated refresh token = %q", got)
	}

	databasePath := filepath.Join(dataDirectory, "blinky-cli", "servers.json")
	raw, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["default"] != server || stored["servers"] == nil {
		t.Fatalf("compatible database schema was not retained: %s", raw)
	}
}

func TestOneTokenSourceCoalescesConcurrentRefreshes(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)

	server := "https://ayato.example"
	if err := SaveEndpoint(server, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := SaveTokens(server, "alice", "old-access", "old-refresh"); err != nil {
		t.Fatal(err)
	}
	source := NewTokenSource(&Endpoint{URL: server, Username: "alice", AccessToken: "old-access"})
	var calls atomic.Int32
	source.refresh = func(context.Context, string, string) (client.TokenPair, error) {
		calls.Add(1)
		return client.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}, nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- source.RefreshIfCurrent(context.Background(), "old-access")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("refresh calls = %d, want 1", calls.Load())
	}
}

func TestRefreshResponseDoesNotOverwriteConcurrentLogin(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)

	server := "https://refresh-login-race.example"
	if err := SaveTokens(server, "alice", "old-access", "old-refresh"); err != nil {
		t.Fatal(err)
	}
	source := NewTokenSource(&Endpoint{URL: server, Username: "alice", AccessToken: "old-access"})
	source.refresh = func(context.Context, string, string) (client.TokenPair, error) {
		if err := SaveTokens(server, "bob", "login-access", "login-refresh"); err != nil {
			return client.TokenPair{}, err
		}
		return client.TokenPair{AccessToken: "rotated-access", RefreshToken: "rotated-refresh", Login: "alice"}, nil
	}

	if err := source.RefreshIfCurrent(context.Background(), "old-access"); err != nil {
		t.Fatal(err)
	}
	if token, _ := source.Token(context.Background()); token != "login-access" {
		t.Fatalf("TokenSource adopted %q, want concurrent login", token)
	}
	snapshot, err := SnapshotCredentials(server)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Username != "bob" || snapshot.Access != "login-access" || snapshot.Refresh != "login-refresh" {
		t.Fatalf("refresh response overwrote concurrent login: %#v", snapshot)
	}
}

func TestSaveTokensFailsWhenRefreshKeyringUnavailable(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	fake.unavailable = true
	useKeyring(t, fake)
	if err := SaveTokens("https://ayato.example", "alice", "access", "refresh"); err == nil {
		t.Fatal("SaveTokens accepted an unpersisted refresh token")
	}
}

func TestSaveEndpointPreservesCredentials(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)
	server := "https://ayato.example"
	if err := SaveTokens(server, "alice", "access", "refresh"); err != nil {
		t.Fatal(err)
	}
	if err := SaveEndpoint(server, "renamed"); err != nil {
		t.Fatal(err)
	}
	endpoint, err := Resolve(server)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Username != "renamed" || endpoint.AccessToken != "access" || loadRefreshTokenValue(server) != "refresh" {
		t.Fatalf("endpoint update changed credentials: %#v refresh=%q", endpoint, loadRefreshTokenValue(server))
	}
}

func TestListEndpointsReturnsOwnedSummaries(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)

	if err := SaveEndpoint("https://b.example", "bob"); err != nil {
		t.Fatal(err)
	}
	if err := SaveEndpoint("https://a.example", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := SetDefault("https://b.example"); err != nil {
		t.Fatal(err)
	}

	endpoints, err := ListEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("endpoint count = %d", len(endpoints))
	}
	if endpoints[0].URL != "https://a.example" || endpoints[0].Username != "alice" || endpoints[0].Default {
		t.Fatalf("first endpoint = %#v", endpoints[0])
	}
	if endpoints[1].URL != "https://b.example" || endpoints[1].Username != "bob" || !endpoints[1].Default {
		t.Fatalf("second endpoint = %#v", endpoints[1])
	}
}

func TestConcurrentCredentialMutationsRemainPairedAndDoNotLoseServers(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)

	const servers = 24
	var wait sync.WaitGroup
	for i := range servers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			server := fmt.Sprintf("https://server-%d.example", i)
			if err := SaveEndpoint(server, "user"); err != nil {
				t.Errorf("SaveEndpoint(%s): %v", server, err)
			}
		}()
	}
	wait.Wait()
	db, err := readDB()
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Endpoints) != servers {
		t.Fatalf("concurrent server updates retained %d entries, want %d", len(db.Endpoints), servers)
	}

	server := "https://paired.example"
	start := make(chan struct{})
	for _, pair := range [][2]string{{"access-a", "refresh-a"}, {"access-b", "refresh-b"}} {
		pair := pair
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			if err := SaveTokens(server, "alice", pair[0], pair[1]); err != nil {
				t.Errorf("SaveTokens: %v", err)
			}
		}()
	}
	close(start)
	wait.Wait()
	endpoint, err := Resolve(server)
	if err != nil {
		t.Fatal(err)
	}
	refresh := loadRefreshTokenValue(server)
	if !((endpoint.AccessToken == "access-a" && refresh == "refresh-a") || (endpoint.AccessToken == "access-b" && refresh == "refresh-b")) {
		t.Fatalf("mixed credential pair: access=%q refresh=%q", endpoint.AccessToken, refresh)
	}
}

func TestClearCredentialsIfCurrentRetainsConcurrentLogin(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)
	server := "https://revoke-race.example"
	if err := SaveTokens(server, "alice", "old-access", "old-refresh"); err != nil {
		t.Fatal(err)
	}

	stale, err := SnapshotCredentials(server)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Access != "old-access" || stale.Refresh != "old-refresh" {
		t.Fatalf("unexpected stale snapshot: %#v", stale)
	}
	if err := SaveTokens(server, "alice", "new-access", "new-refresh"); err != nil {
		t.Fatal(err)
	}
	cleared, err := ClearCredentialsIfCurrent(stale, true)
	if err != nil {
		t.Fatal(err)
	}
	if cleared {
		t.Fatal("stale revoke cleared a newer login")
	}
	endpoint, err := Resolve(server)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.AccessToken != "new-access" || loadRefreshTokenValue(server) != "new-refresh" {
		t.Fatalf("new login was not retained: access=%q refresh=%q", endpoint.AccessToken, loadRefreshTokenValue(server))
	}

	current, err := SnapshotCredentials(server)
	if err != nil {
		t.Fatal(err)
	}
	cleared, err = ClearCredentialsIfCurrent(current, true)
	if err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Fatal("current credential pair was not cleared")
	}
	endpoint, err = Resolve(server)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.AccessToken != "" || loadRefreshTokenValue(server) != "" {
		t.Fatalf("credentials remain after matching clear: access=%q refresh=%q", endpoint.AccessToken, loadRefreshTokenValue(server))
	}
}

func TestFailedOAuthReplacementLeavesDurableDisabledMode(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)
	server := "https://crash-safe.example"
	if err := SaveTokens(server, "alice", "old-access", "old-refresh"); err != nil {
		t.Fatal(err)
	}
	fake.unavailable = true
	if err := SaveTokens(server, "bob", "new-access", "new-refresh"); err == nil {
		t.Fatal("replacement unexpectedly succeeded with unavailable keyring")
	}
	entry, err := readDB()
	if err != nil {
		t.Fatal(err)
	}
	if got := loadAccessTokenValue(server, entry.Endpoints[server].AccessToken); got != "" {
		t.Fatalf("failed replacement re-enabled access token %q", got)
	}
	if got := loadRefreshTokenValue(server); got != "" {
		t.Fatalf("failed replacement re-enabled refresh token %q", got)
	}
}

func TestCorruptCredentialStateFailsClosedForFileAndKeyring(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	fake := newFakeKeyring()
	useKeyring(t, fake)
	server := "https://corrupt-state.example"
	if _, err := storeAccessToken(server, "keyring-access"); err != nil {
		t.Fatal(err)
	}
	path, err := credentialStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadAccessTokenValue(server, "file-access"); got != "" {
		t.Fatalf("corrupt state failed open to %q", got)
	}
	if got := loadRefreshTokenValue(server); got != "" {
		t.Fatalf("corrupt state enabled refresh %q", got)
	}
}
