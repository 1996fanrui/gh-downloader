package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/1996fanrui/gh-downloader/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	db postgresDB
}

func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{db: pgxPoolDB{pool: pool}}
}

func (s *PostgresStore) Init(ctx context.Context) error {
	if s.db == nil {
		return errors.New("postgres store database is required")
	}
	return s.db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS repositories (
	owner text NOT NULL,
	name text NOT NULL,
	data jsonb NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (owner, name),
	CHECK (owner <> ''),
	CHECK (name <> '')
)`)
}

func (s *PostgresStore) List(ctx context.Context) ([]domain.Repository, error) {
	rows, err := s.db.Query(ctx, `
SELECT owner, name, data
FROM repositories
ORDER BY owner, name`)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	defer rows.Close()

	var repos []domain.Repository
	for rows.Next() {
		repo, err := scanRepository(rows)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list repositories rows: %w", err)
	}
	return repos, nil
}

func (s *PostgresStore) Get(ctx context.Context, owner string, repo string) (domain.Repository, bool, error) {
	row := s.db.QueryRow(ctx, `
SELECT owner, name, data
FROM repositories
WHERE owner = $1 AND name = $2`, owner, repo)
	item, err := scanRepository(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Repository{}, false, nil
	}
	if err != nil {
		return domain.Repository{}, false, err
	}
	return item, true, nil
}

func (s *PostgresStore) Save(ctx context.Context, repo domain.Repository) error {
	if err := validateRepository(repo); err != nil {
		return err
	}
	body, err := json.Marshal(repo)
	if err != nil {
		return fmt.Errorf("encode repository %s/%s: %w", repo.Owner, repo.Name, err)
	}
	if err := s.db.Exec(ctx, `
INSERT INTO repositories (owner, name, data, updated_at)
VALUES ($1, $2, $3::jsonb, now())
ON CONFLICT (owner, name)
DO UPDATE SET data = EXCLUDED.data, updated_at = now()`, repo.Owner, repo.Name, string(body)); err != nil {
		return fmt.Errorf("save repository %s/%s: %w", repo.Owner, repo.Name, err)
	}
	return nil
}

func scanRepository(scanner rowScanner) (domain.Repository, error) {
	var owner string
	var name string
	var body []byte
	if err := scanner.Scan(&owner, &name, &body); err != nil {
		return domain.Repository{}, fmt.Errorf("scan repository row: %w", err)
	}
	var repo domain.Repository
	if err := json.Unmarshal(body, &repo); err != nil {
		return domain.Repository{}, fmt.Errorf("decode repository %s/%s: %w", owner, name, err)
	}
	if err := validateRepository(repo); err != nil {
		return domain.Repository{}, fmt.Errorf("repository %s/%s has invalid data: %w", owner, name, err)
	}
	if repo.Owner != owner || repo.Name != name {
		return domain.Repository{}, fmt.Errorf("repository row key %s/%s does not match payload %s/%s", owner, name, repo.Owner, repo.Name)
	}
	return repo, nil
}

func validateRepository(repo domain.Repository) error {
	if repo.Owner == "" {
		return errors.New("repository owner is required")
	}
	if repo.Name == "" {
		return errors.New("repository name is required")
	}
	if repo.FullName != "" && repo.FullName != repo.Owner+"/"+repo.Name {
		return fmt.Errorf("repository full name %q does not match owner/name %s/%s", repo.FullName, repo.Owner, repo.Name)
	}
	return nil
}

type postgresDB interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) (rowsScanner, error)
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
}

type rowScanner interface {
	Scan(dest ...any) error
}

type rowsScanner interface {
	rowScanner
	Next() bool
	Err() error
	Close()
}

type pgxPoolDB struct {
	pool *pgxpool.Pool
}

func (db pgxPoolDB) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}

func (db pgxPoolDB) Query(ctx context.Context, sql string, args ...any) (rowsScanner, error) {
	return db.pool.Query(ctx, sql, args...)
}

func (db pgxPoolDB) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return db.pool.QueryRow(ctx, sql, args...)
}
