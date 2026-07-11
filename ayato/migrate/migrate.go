package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type Stores struct {
	KV   kv.Store
	Blob blob.Store
}

func (s *Stores) ObjectMover() (blob.ObjectMover, error) {
	m, ok := s.Blob.(blob.ObjectMover)
	if !ok {
		return nil, errors.New("blob store does not support raw object moves")
	}
	return m, nil
}

// Migration is one versioned, forward-only step. Expand adds the new layout while the
// old one still serves; Contract removes the old layout after the new binary ships.
// Both must be idempotent so an interrupted run resumes by re-running.
type Migration interface {
	Version() int
	Name() string
	Expand(ctx context.Context, s *Stores) error
	Contract(ctx context.Context, s *Stores) error
}

type Phase string

const (
	PhaseExpand   Phase = "expand"
	PhaseContract Phase = "contract"
)

type RunOptions struct {
	Phase  Phase
	To     int // 0 runs every registered migration
	DryRun bool
}

type Result struct {
	Phase   Phase
	Applied []int
	Skipped []int
}

// Run applies the phase to each migration in version order. Markers make it idempotent
// and resumable; Contract advances the layout version (the old layout is now gone).
func Run(ctx context.Context, s *Stores, migrations []Migration, o RunOptions) (Result, error) {
	sorted := append([]Migration(nil), migrations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version() < sorted[j].Version() })

	res := Result{Phase: o.Phase}
	for _, m := range sorted {
		v := m.Version()
		if o.To > 0 && v > o.To {
			continue
		}
		applied, err := runOne(ctx, s, m, o, &res)
		if err != nil {
			return res, err
		}
		if applied {
			res.Applied = append(res.Applied, v)
		}
	}
	return res, nil
}

func runOne(ctx context.Context, s *Stores, m Migration, o RunOptions, res *Result) (bool, error) {
	v := m.Version()
	switch o.Phase {
	case PhaseExpand:
		if done, err := hasMarker(s.KV, "expanded", v); err != nil || done {
			res.Skipped = appendIf(res.Skipped, v, done)
			return false, err
		}
		slog.Info("migrate expand", "version", v, "name", m.Name(), "dryRun", o.DryRun)
		if o.DryRun {
			return true, nil
		}
		if err := m.Expand(ctx, s); err != nil {
			return false, errors.WrapErr(err, fmt.Sprintf("expand %d", v))
		}
		return true, setMarker(s.KV, "expanded", v)
	case PhaseContract:
		if exp, err := hasMarker(s.KV, "expanded", v); err != nil || !exp {
			return false, err // never contract a layout that was not expanded
		}
		if done, err := hasMarker(s.KV, "contracted", v); err != nil || done {
			res.Skipped = appendIf(res.Skipped, v, done)
			return false, err
		}
		slog.Info("migrate contract", "version", v, "name", m.Name(), "dryRun", o.DryRun)
		if o.DryRun {
			return true, nil
		}
		if err := m.Contract(ctx, s); err != nil {
			return false, errors.WrapErr(err, fmt.Sprintf("contract %d", v))
		}
		if err := setMarker(s.KV, "contracted", v); err != nil {
			return false, err
		}
		return true, WriteLayout(s.KV, v)
	default:
		return false, fmt.Errorf("unknown migration phase %q", o.Phase)
	}
}

func appendIf(s []int, v int, cond bool) []int {
	if cond {
		return append(s, v)
	}
	return s
}

type MigrationStatus struct {
	Version    int
	Name       string
	Expanded   bool
	Contracted bool
}

type Status struct {
	Layout     int
	Migrations []MigrationStatus
}

func Statuses(s kv.Store, migrations []Migration) (Status, error) {
	layout, err := ReadLayout(s)
	if err != nil {
		return Status{}, err
	}
	out := Status{Layout: layout}
	for _, m := range migrations {
		exp, err := hasMarker(s, "expanded", m.Version())
		if err != nil {
			return Status{}, err
		}
		con, err := hasMarker(s, "contracted", m.Version())
		if err != nil {
			return Status{}, err
		}
		out.Migrations = append(out.Migrations, MigrationStatus{
			Version: m.Version(), Name: m.Name(), Expanded: exp, Contracted: con,
		})
	}
	return out, nil
}

func markerKey(kind string, v int) string { return kind + "/" + strconv.Itoa(v) }

func hasMarker(s kv.Store, kind string, v int) (bool, error) {
	_, err := s.Get(metaNS, markerKey(kind, v))
	if errors.Is(err, kv.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func setMarker(s kv.Store, kind string, v int) error {
	return s.Set(metaNS, markerKey(kind, v), []byte("1"), 0)
}
