package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type MALIDCache struct {
	db *sql.DB
}

func NewMALIDCache() (*MALIDCache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".anilix")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dbPath := filepath.Join(cacheDir, "malid_cache.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS malid_allanime_map (
			mal_id INTEGER PRIMARY KEY,
			allanime_id TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &MALIDCache{db: db}, nil
}

func (c *MALIDCache) Get(malID int) (string, error) {
	var AllanimeID string
	err := c.db.QueryRow(
		"SELECT Allanime_id FROM malid_allanime_map WHERE mal_id = ?",
		malID,
	).Scan(&AllanimeID)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("not found")
	}
	if err != nil {
		return "", err
	}

	return AllanimeID, nil
}

func (c *MALIDCache) Set(malID int, AllanimeID string) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO malid_allanime_map (mal_id, Allanime_id, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)`,
		malID, AllanimeID,
	)
	return err
}

func (c *MALIDCache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}
