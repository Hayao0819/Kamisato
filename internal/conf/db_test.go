package conf

import "testing"

func TestSqlConfigDSN(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SqlConfig
		want    string
		wantErr bool
	}{
		{
			name: "postgres",
			cfg:  SqlConfig{Driver: "postgres", Host: "localhost", User: "admin", Database: "ayato"},
			want: "host=localhost port=5432 user=admin password= dbname=ayato",
		},
		{
			name: "postgres with additional dsn",
			cfg:  SqlConfig{Driver: "postgres", Host: "localhost", User: "admin", Database: "ayato", AdditionalDSN: "sslmode=require"},
			want: "host=localhost port=5432 user=admin password= dbname=ayato sslmode=require",
		},
		{
			name: "mysql",
			cfg:  SqlConfig{Driver: "mysql", Host: "db", User: "root", Password: "secret", Database: "ayato"},
			want: "root:secret@tcp(db:3306)/ayato",
		},
		{
			name: "sqlite",
			cfg:  SqlConfig{Driver: "sqlite", Database: "/var/lib/ayato/ayato.db"},
			want: "/var/lib/ayato/ayato.db",
		},
		{
			// A password with reserved bytes must not corrupt the DSN.
			name: "mysql password with reserved chars",
			cfg:  SqlConfig{Driver: "mysql", Host: "db", User: "root", Password: "p@ss:w/rd", Database: "ayato"},
			want: "root:p@ss:w/rd@tcp(db:3306)/ayato",
		},
		{
			name: "postgres password with space and quote",
			cfg:  SqlConfig{Driver: "postgres", Host: "localhost", User: "admin", Password: "a b'c", Database: "ayato"},
			want: `host=localhost port=5432 user=admin password='a b\'c' dbname=ayato`,
		},
		{
			name:    "unsupported driver",
			cfg:     SqlConfig{Driver: "oracle", Host: "h", User: "u", Database: "d"},
			wantErr: true,
		},
		{
			name:    "postgres missing host",
			cfg:     SqlConfig{Driver: "postgres", User: "u", Database: "d"},
			wantErr: true,
		},
		{
			name:    "sqlite missing database",
			cfg:     SqlConfig{Driver: "sqlite"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.DSN()
			if (err != nil) != tt.wantErr {
				t.Fatalf("DSN() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("DSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAyatoConfigRepoHelpers(t *testing.T) {
	cfg := AyatoConfig{
		Repos: []BinRepoConfig{
			{Name: "core"},
			{Name: "extra"},
		},
	}

	names := cfg.RepoNames()
	if len(names) != 2 || names[0] != "core" || names[1] != "extra" {
		t.Errorf("RepoNames() = %v, want [core extra]", names)
	}

	if r := cfg.Repo("extra"); r == nil {
		t.Errorf("Repo(extra) = %v, want a config", r)
	}
	if r := cfg.Repo("missing"); r != nil {
		t.Errorf("Repo(missing) = %v, want nil", r)
	}
}
