package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type recordingExecutor struct {
	queries []string
}

func (r *recordingExecutor) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	r.queries = append(r.queries, strings.ToLower(strings.Join(strings.Fields(query), " ")))
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestSeedRolesOnlyBootstrapsRBACTables(t *testing.T) {
	executor := &recordingExecutor{}
	if err := seedRoles(context.Background(), executor); err != nil {
		t.Fatalf("seedRoles returned error: %v", err)
	}
	if len(executor.queries) == 0 {
		t.Fatal("seedRoles executed no statements")
	}

	for _, query := range executor.queries {
		if !strings.Contains(query, "insert into roles") && !strings.Contains(query, "insert into role_permissions") {
			t.Fatalf("bootstrap wrote outside RBAC tables: %s", query)
		}
		for _, productTable := range []string{"events", "places", "event_participants", "event_purchases", "profiles", "tickets", "calendars"} {
			if strings.Contains(query, "insert into "+productTable) {
				t.Fatalf("bootstrap wrote product table %q: %s", productTable, query)
			}
		}
	}
}
