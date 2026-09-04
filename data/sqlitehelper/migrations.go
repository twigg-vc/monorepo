package sqlitehelper

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func (s *sqlite) init(migrationsFsys fs.FS) error {
	if migrationsFsys == nil {
		return nil
	}
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			filename TEXT PRIMARY KEY,
			hash     TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	// Collect and sort .sql files so migrations always run in filename order.
	const maxFiles = 5_000 // Just to avoid endless recursion
	var files []string
	err = fs.WalkDir(migrationsFsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if len(files) > maxFiles {
			return fmt.Errorf("max number files (%d) achieved", len(files))
		}
		if filepath.Ext(path) == ".sql" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no migration files found")
	}
	sort.Strings(files)

	for _, file := range files {
		if err := validateMigrationFilename(file); err != nil {
			return err
		}
		content, err := fs.ReadFile(migrationsFsys, file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		if err := applyMigration(s.db, file, content); err != nil {
			return err
		}
	}

	return nil
}

// validMigrationName matches files like 0000001_create_users.sql
var validMigrationName = regexp.MustCompile(`^\d{7}_[a-z0-9_]+\.sql$`)

func validateMigrationFilename(path string) error {
	filename := filepath.Base(path)
	if !validMigrationName.MatchString(filename) {
		return fmt.Errorf(
			"migration filename %q does not match expected pattern NNNNNNN_description.sql (e.g. 0000001_create_users.sql)",
			filename,
		)
	}
	return nil
}

// applyMigration runs a single migration inside a transaction and records it.
// All migrations are intentionally transactional. Statements that SQLite
// forbids inside transactions are not supported - an errow will be returned
// if one is attempted.
func applyMigration(db *sql.DB, file string, content []byte) error {
	hash := fmt.Sprintf("%x", sha256.Sum256(content))

	var existingHash string
	err := db.QueryRow(
		"SELECT hash FROM migrations WHERE filename = ?", file,
	).Scan(&existingHash)

	if err == nil {
		if existingHash != hash {
			return fmt.Errorf("migration %s: %w", file, ErrTamperedMigration)
		}
		return nil // already applied, skip
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check %s: %w", file, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", file, err)
	}
	defer tx.Rollback() // safe no-op after Commit

	// Split by semicolon to support multiple queries in one file - the used
	// SQLite driver doesnt suppoert it
	statements := strings.Split(string(content), ";")
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue // Skip trailing or empty statements
		}

		if _, err := tx.Exec(trimmed); err != nil {
			return fmt.Errorf("execute statement in %s: %w", file, err)
		}
	}
	if _, err := tx.Exec(
		"INSERT INTO migrations (filename, hash) VALUES (?,?)", file, hash,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", file, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", file, err)
	}
	return nil
}
