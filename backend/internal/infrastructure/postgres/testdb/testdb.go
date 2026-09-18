package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const isolatedFlag = "NIMNEAR_TEST_DB_ISOLATED"

// UniqueHash returns a 64-character transaction hash that will not collide
// with other parallel PostgreSQL packages sharing consumed_nimiq_transactions.
func UniqueHash(label string) string {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err)
	}
	raw := strings.ToLower(strings.TrimSpace(label)) + hex.EncodeToString(nonce[:])
	if len(raw) >= 64 {
		return raw[:64]
	}
	return raw + strings.Repeat("0", 64-len(raw))
}

// Open connects to the isolated PostgreSQL database used by destructive
// integration tests. Tests skip when NIMNEAR_TEST_DATABASE_URL is unset.
// They fail closed if the DSN does not name a *_test database or if
// NIMNEAR_TEST_DB_ISOLATED is not true.
func Open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("NIMNEAR_TEST_DATABASE_URL"))
	if dsn == "" {
		if inCI() {
			t.Fatal("NIMNEAR_TEST_DATABASE_URL is required in CI; isolated PostgreSQL tests must not skip")
		}
		t.Skip("NIMNEAR_TEST_DATABASE_URL is not set")
	}
	if err := isolationError(dsn, os.Getenv(isolatedFlag)); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	return pool
}

func isolationError(dsn, isolated string) error {
	name, err := databaseName(dsn)
	if err != nil {
		return fmt.Errorf("parse NIMNEAR_TEST_DATABASE_URL: %w", err)
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return fmt.Errorf("refusing destructive tests against an unnamed database")
	}
	if !strings.HasSuffix(lower, "_test") {
		return fmt.Errorf("refusing destructive tests against database %q; name must end with _test", name)
	}
	if lower == "nimnear" || strings.Contains(lower, "prod") {
		return fmt.Errorf("refusing destructive tests against database %q", name)
	}
	if strings.ToLower(strings.TrimSpace(isolated)) != "true" {
		return fmt.Errorf("%s=true is required before running destructive PostgreSQL tests", isolatedFlag)
	}
	return nil
}

func inCI() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") || strings.EqualFold(strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")), "true")
}

func databaseName(dsn string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if name == "" {
		if dbname := parsed.Query().Get("dbname"); dbname != "" {
			return dbname, nil
		}
	}
	return name, nil
}
