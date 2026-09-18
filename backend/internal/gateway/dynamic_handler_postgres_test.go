package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/apimanagement/model"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
)

func TestDynamicHandlerAllowlistedCreateParameterizesValues(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS gateway_test_items (
			id UUID PRIMARY KEY,
			organization_id UUID NOT NULL,
			app_id UUID NOT NULL,
			title TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM gateway_test_items")
	})

	resolver := NewDynamicHandlerResolver(nil, nil, pool).WithAllowedTables([]string{"gateway_test_items"})
	orgID, appID := uuid.New(), uuid.New()
	endpoint := &model.Endpoint{
		BackendService: "catalog-service",
		BackendAction:  "create",
		Schema:         []byte(`{"table_name":"gateway_test_items","allowed_fields":["title"]}`),
	}
	title := `safe value; DROP TABLE users`
	req := httptestGatewayRequest(http.MethodPost, "/api/v1/items", bytes.NewReader([]byte(`{"title":`+jsonString(title)+`}`)), orgID, appID)
	resp, err := resolver.ResolveHandler(req.Context(), endpoint, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil || data["title"] != title {
		t.Fatalf("stored title = %#v", payload)
	}

	var usersExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')`).Scan(&usersExists); err != nil {
		t.Fatal(err)
	}
	if !usersExists {
		t.Fatal("users table missing; test must not execute destructive SQL")
	}
}
