package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/apimanagement/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

func TestDynamicHandlerRejectsMaliciousAndDeniedTablesWithoutQuerying(t *testing.T) {
	resolver := NewDynamicHandlerResolver(nil, nil, nil).WithAllowedTables([]string{"gateway_test_items"})
	orgID, appID := uuid.New(), uuid.New()
	for _, table := range []string{
		`users; DROP TABLE users`,
		"users--",
		"public.users",
		`"user"`,
		"foo bar",
		"foo)",
		"users",
		"event_purchases",
		"payment_requests",
	} {
		endpoint := &model.Endpoint{
			BackendService: "catalog-service",
			BackendAction:  "list",
			Schema:         []byte(`{"table_name":` + jsonString(table) + `}`),
		}
		req := httptestGatewayRequest(http.MethodGet, "/api/v1/items", nil, orgID, appID)
		resp, err := resolver.ResolveHandler(req.Context(), endpoint, req)
		if err != nil {
			t.Fatalf("table %q: %v", table, err)
		}
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("table %q status=%d body=%s", table, resp.StatusCode, body)
		}
	}
}

func TestDynamicHandlerRejectsUndeclaredColumns(t *testing.T) {
	resolver := NewDynamicHandlerResolver(nil, nil, nil).WithAllowedTables([]string{"gateway_test_items"})
	orgID, appID := uuid.New(), uuid.New()
	endpoint := &model.Endpoint{
		BackendService: "catalog-service",
		BackendAction:  "create",
		Schema:         []byte(`{"table_name":"gateway_test_items","allowed_fields":["title"]}`),
	}
	body := bytes.NewReader([]byte(`{"password_hash":"x","transaction_hash":"y"}`))
	req := httptestGatewayRequest(http.MethodPost, "/api/v1/items", body, orgID, appID)
	resp, err := resolver.ResolveHandler(req.Context(), endpoint, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, payload)
	}
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func httptestGatewayRequest(method, path string, body io.Reader, orgID, appID uuid.UUID) *http.Request {
	req, _ := http.NewRequest(method, path, body)
	req.Header.Set("X-App-ID", appID.String())
	ctx := context.WithValue(req.Context(), middleware.ContextKeyOrganizationID, orgID)
	return req.WithContext(ctx)
}

func TestQuotedSQLDoesNotEmbedUserValues(t *testing.T) {
	query := `INSERT INTO "gateway_test_items" ("id", "organization_id", "app_id", "created_at", "updated_at", "title") VALUES ($1, $2, $3, $4, $5, $6)`
	if strings.Contains(query, "DROP") || strings.Contains(query, "mug") {
		t.Fatal("query unexpectedly contained a user value")
	}
	if !strings.Contains(query, "$6") {
		t.Fatal("values must remain parameterized")
	}
}
