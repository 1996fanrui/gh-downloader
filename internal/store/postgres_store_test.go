package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/1996fanrui/gh-downloader/internal/domain"
	"github.com/jackc/pgx/v5"
)

func TestPostgresStoreInitCreatesRepositoriesTable(t *testing.T) {
	db := &fakePostgresDB{}
	store := &PostgresStore{db: db}

	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if len(db.execs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(db.execs))
	}
	if !strings.Contains(db.execs[0].sql, "CREATE TABLE IF NOT EXISTS repositories") {
		t.Fatalf("Init SQL does not create repositories table: %s", db.execs[0].sql)
	}
	if !strings.Contains(db.execs[0].sql, "PRIMARY KEY (owner, name)") {
		t.Fatalf("Init SQL does not define owner/name primary key: %s", db.execs[0].sql)
	}
	if !strings.Contains(db.execs[0].sql, "data jsonb NOT NULL") {
		t.Fatalf("Init SQL does not persist repository JSON payload: %s", db.execs[0].sql)
	}
}

func TestPostgresStoreSaveUpsertsRepositoryJSON(t *testing.T) {
	db := &fakePostgresDB{}
	store := &PostgresStore{db: db}
	repo := testRepository()

	if err := store.Save(context.Background(), repo); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if len(db.execs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(db.execs))
	}
	call := db.execs[0]
	if !strings.Contains(call.sql, "ON CONFLICT (owner, name)") {
		t.Fatalf("Save SQL does not upsert by owner/name: %s", call.sql)
	}
	if !strings.Contains(call.sql, "$3::jsonb") {
		t.Fatalf("Save SQL does not cast JSON parameter to jsonb: %s", call.sql)
	}
	if len(call.args) != 3 {
		t.Fatalf("Save args = %d, want 3", len(call.args))
	}
	if call.args[0] != repo.Owner || call.args[1] != repo.Name {
		t.Fatalf("Save key args = %#v, want owner/name", call.args[:2])
	}
	body, ok := call.args[2].(string)
	if !ok {
		t.Fatalf("Save JSON arg type = %T, want string", call.args[2])
	}
	var stored domain.Repository
	if err := json.Unmarshal([]byte(body), &stored); err != nil {
		t.Fatalf("stored JSON is invalid: %v", err)
	}
	if stored.Owner != repo.Owner || stored.Name != repo.Name || len(stored.Versions) != 1 {
		t.Fatalf("stored repository = %#v, want full repository payload", stored)
	}
}

func TestPostgresStoreSaveRejectsInvalidRepository(t *testing.T) {
	db := &fakePostgresDB{}
	store := &PostgresStore{db: db}

	err := store.Save(context.Background(), domain.Repository{Name: "codex"})
	if err == nil {
		t.Fatal("Save accepted repository without owner")
	}
	if len(db.execs) != 0 {
		t.Fatalf("exec count = %d, want 0", len(db.execs))
	}
}

func TestPostgresStoreGetReturnsRepository(t *testing.T) {
	repo := testRepository()
	db := &fakePostgresDB{
		row: fakeRow{values: repositoryRow(t, repo.Owner, repo.Name, repo)},
	}
	store := &PostgresStore{db: db}

	got, found, err := store.Get(context.Background(), repo.Owner, repo.Name)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !found {
		t.Fatal("Get found = false, want true")
	}
	if got.FullName != repo.FullName || len(got.Versions) != 1 {
		t.Fatalf("Get repository = %#v, want %#v", got, repo)
	}
	if len(db.queryRows) != 1 {
		t.Fatalf("query row count = %d, want 1", len(db.queryRows))
	}
	if db.queryRows[0].args[0] != repo.Owner || db.queryRows[0].args[1] != repo.Name {
		t.Fatalf("Get args = %#v, want owner/name", db.queryRows[0].args)
	}
}

func TestPostgresStoreGetReturnsNotFound(t *testing.T) {
	db := &fakePostgresDB{row: fakeRow{err: pgx.ErrNoRows}}
	store := &PostgresStore{db: db}

	_, found, err := store.Get(context.Background(), "openai", "codex")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if found {
		t.Fatal("Get found = true, want false")
	}
}

func TestPostgresStoreGetRejectsMismatchedPayload(t *testing.T) {
	repo := testRepository()
	db := &fakePostgresDB{
		row: fakeRow{values: repositoryRow(t, "openai", "wrong", repo)},
	}
	store := &PostgresStore{db: db}

	_, _, err := store.Get(context.Background(), "openai", "wrong")
	if err == nil {
		t.Fatal("Get accepted row whose key does not match payload")
	}
}

func TestPostgresStoreListReturnsOrderedRepositories(t *testing.T) {
	first := testRepository()
	second := testRepository()
	second.Owner = "zed"
	second.Name = "tool"
	second.FullName = "zed/tool"
	db := &fakePostgresDB{
		rows: &fakeRows{rows: [][]any{
			repositoryRow(t, first.Owner, first.Name, first),
			repositoryRow(t, second.Owner, second.Name, second),
		}},
	}
	store := &PostgresStore{db: db}

	repos, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("List count = %d, want 2", len(repos))
	}
	if repos[0].FullName != first.FullName || repos[1].FullName != second.FullName {
		t.Fatalf("List repos = %#v", repos)
	}
	if len(db.queries) != 1 || !strings.Contains(db.queries[0].sql, "ORDER BY owner, name") {
		t.Fatalf("List SQL should order by owner/name: %#v", db.queries)
	}
}

func testRepository() domain.Repository {
	return domain.Repository{
		Owner:        "openai",
		Name:         "codex",
		FullName:     "openai/codex",
		Description:  "Coding agent",
		HTMLURL:      "https://github.com/openai/codex",
		LastSyncedAt: time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC),
		Versions: []domain.ReleaseVersion{
			{
				Tag:         "v1.0.0",
				Name:        "v1.0.0",
				PublishedAt: time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC),
				AssetTotal:  1,
				Assets: []domain.ReleaseAsset{
					{Name: "codex-linux-x64.tar.gz", Size: 42, DownloadURL: "https://example.test/codex-linux-x64.tar.gz"},
				},
				PlatformMap: map[domain.Platform]*domain.PlatformSelection{
					domain.PlatformLinuxX64: {Asset: "codex-linux-x64.tar.gz"},
				},
			},
		},
	}
}

func repositoryRow(t *testing.T, owner string, name string, repo domain.Repository) []any {
	t.Helper()
	body, err := json.Marshal(repo)
	if err != nil {
		t.Fatalf("marshal repository: %v", err)
	}
	return []any{owner, name, body}
}

type fakePostgresDB struct {
	execs     []fakeCall
	queries   []fakeCall
	queryRows []fakeCall
	rows      rowsScanner
	row       rowScanner
	execErr   error
	queryErr  error
}

func (db *fakePostgresDB) Exec(_ context.Context, sql string, args ...any) error {
	db.execs = append(db.execs, fakeCall{sql: sql, args: args})
	return db.execErr
}

func (db *fakePostgresDB) Query(_ context.Context, sql string, args ...any) (rowsScanner, error) {
	db.queries = append(db.queries, fakeCall{sql: sql, args: args})
	if db.queryErr != nil {
		return nil, db.queryErr
	}
	if db.rows == nil {
		return &fakeRows{}, nil
	}
	return db.rows, nil
}

func (db *fakePostgresDB) QueryRow(_ context.Context, sql string, args ...any) rowScanner {
	db.queryRows = append(db.queryRows, fakeCall{sql: sql, args: args})
	if db.row == nil {
		return fakeRow{err: pgx.ErrNoRows}
	}
	return db.row
}

type fakeCall struct {
	sql  string
	args []any
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScanValues(dest, r.values)
}

type fakeRows struct {
	rows   [][]any
	index  int
	err    error
	closed bool
}

func (r *fakeRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) {
		return errors.New("scan called without current row")
	}
	return assignScanValues(dest, r.rows[r.index-1])
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) Close() {
	r.closed = true
}

func assignScanValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return errors.New("destination count does not match value count")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			value, ok := values[i].(string)
			if !ok {
				return errors.New("value is not string")
			}
			*target = value
		case *[]byte:
			value, ok := values[i].([]byte)
			if !ok {
				return errors.New("value is not []byte")
			}
			*target = value
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
