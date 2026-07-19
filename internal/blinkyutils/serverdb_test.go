package blinkyutils

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	blinkyutil "github.com/BrenekH/blinky/cmd/blinky/util"
)

func TestRegistryCodecMatchesBlinkySchema(t *testing.T) {
	registry := Registry{
		Default: "https://ayato.example",
		Endpoints: map[string]StoredEndpoint{
			"https://ayato.example": {
				Username:    "alice",
				AccessToken: "token",
			},
		},
	}
	raw, err := EncodeRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}

	var upstream blinkyutil.ServerDB
	if err := json.Unmarshal(raw, &upstream); err != nil {
		t.Fatal(err)
	}
	if upstream.DefaultServer != registry.Default {
		t.Fatalf("default = %q", upstream.DefaultServer)
	}
	server := upstream.Servers["https://ayato.example"]
	if server.Username != "alice" || server.Password != "token" {
		t.Fatalf("upstream server = %#v", server)
	}

	upstreamRaw, err := json.Marshal(upstream)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRegistry(upstreamRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, registry) {
		t.Fatalf("decoded registry = %#v", decoded)
	}
}

func TestUpstreamServerTypesHaveReviewedShape(t *testing.T) {
	serverType := reflect.TypeOf(blinkyutil.Server{})
	assertField(t, serverType, 0, "Username", "username", reflect.TypeOf(""))
	assertField(t, serverType, 1, "Password", "password", reflect.TypeOf(""))
	if serverType.NumField() != 2 {
		t.Fatalf("upstream Server fields = %d, want 2", serverType.NumField())
	}

	dbType := reflect.TypeOf(blinkyutil.ServerDB{})
	assertField(t, dbType, 0, "DefaultServer", "default", reflect.TypeOf(""))
	assertField(t, dbType, 1, "Servers", "servers", reflect.TypeOf(map[string]blinkyutil.Server{}))
	if dbType.NumField() != 2 {
		t.Fatalf("upstream ServerDB fields = %d, want 2", dbType.NumField())
	}
}

func assertField(t *testing.T, typ reflect.Type, index int, name, jsonName string, fieldType reflect.Type) {
	t.Helper()
	if typ.NumField() <= index {
		t.Fatalf("%s field %d is missing", typ.Name(), index)
	}
	field := typ.Field(index)
	if field.Name != name || field.Tag.Get("json") != jsonName || field.Type != fieldType {
		t.Fatalf("%s field %d = %s %s %q", typ.Name(), index, field.Name, field.Type, field.Tag.Get("json"))
	}
}

func TestRegistryPathUsesBlinkyDataDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	path, err := RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	want := root + "/blinky-cli/servers.json"
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestRegistryPathMatchesUpstream(t *testing.T) {
	root := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestWriteUpstreamRegistry$")
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "XDG_DATA_HOME=") || strings.HasPrefix(value, "KAMISATO_BLINKY_PATH_HELPER=") {
			continue
		}
		command.Env = append(command.Env, value)
	}
	command.Env = append(command.Env, "XDG_DATA_HOME="+root, "KAMISATO_BLINKY_PATH_HELPER=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("write upstream registry: %v: %s", err, output)
	}

	t.Setenv("XDG_DATA_HOME", root)
	path, err := RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("upstream registry not found at adapter path %q: %v", path, err)
	}
}

func TestWriteUpstreamRegistry(t *testing.T) {
	if os.Getenv("KAMISATO_BLINKY_PATH_HELPER") != "1" {
		return
	}
	if err := blinkyutil.SaveServerDB(blinkyutil.NewServerDB()); err != nil {
		t.Fatal(err)
	}
}
