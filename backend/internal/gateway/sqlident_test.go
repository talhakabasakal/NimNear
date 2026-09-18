package gateway

import (
	"strings"
	"testing"

	"github.com/masterfabric-go/masterfabric/internal/domain/apimanagement/model"
)

func TestValidateSQLIdentifierRejectsMaliciousNames(t *testing.T) {
	for _, name := range []string{
		`users; DROP TABLE users`,
		"users--",
		"public.users",
		`"user"`,
		"foo bar",
		"foo)",
		"id = 1",
		"",
		"1users",
	} {
		if err := validateSQLIdentifier(name); err == nil {
			t.Fatalf("accepted %q", name)
		}
		if _, err := quoteSQLIdentifier(name); err == nil {
			t.Fatalf("quoted %q", name)
		}
	}
	quoted, err := quoteSQLIdentifier("catalog_items")
	if err != nil || quoted != `"catalog_items"` {
		t.Fatalf("quote catalog_items = %q err=%v", quoted, err)
	}
}

func TestAuthorizeGatewayTableFailsClosed(t *testing.T) {
	if err := authorizeGatewayTable("users", map[string]struct{}{"users": {}}); err == nil {
		t.Fatal("denied security table was allowed")
	}
	if err := authorizeGatewayTable("orders", nil); err == nil {
		t.Fatal("empty allowlist must fail closed")
	}
	if err := authorizeGatewayTable("orders", map[string]struct{}{"catalog_items": {}}); err == nil {
		t.Fatal("undeclared table was allowed")
	}
	if err := authorizeGatewayTable("orders", map[string]struct{}{"orders": {}}); err != nil {
		t.Fatalf("allowlisted table rejected: %v", err)
	}
}

func TestAuthorizedColumnsRejectUndeclaredAndReserved(t *testing.T) {
	declared := []string{"title", "quantity"}
	if _, _, err := authorizedColumns(map[string]interface{}{"password_hash": "x"}, declared); err == nil {
		t.Fatal("password_hash was accepted")
	}
	if _, _, err := authorizedColumns(map[string]interface{}{"transaction_hash": "x"}, declared); err == nil {
		t.Fatal("transaction_hash was accepted")
	}
	if _, _, err := authorizedColumns(map[string]interface{}{"unknown": 1}, declared); err == nil {
		t.Fatal("undeclared column was accepted")
	}
	cols, values, err := authorizedColumns(map[string]interface{}{"title": "mug"}, declared)
	if err != nil || len(cols) != 1 || cols[0] != `"title"` || values[0] != "mug" {
		t.Fatalf("authorized column = %v %v err=%v", cols, values, err)
	}
}

func TestDeclaredGatewayFieldsPreferAllowlist(t *testing.T) {
	endpoint := &model.Endpoint{Schema: []byte(`{"table_name":"orders","allowed_fields":["sku","qty"],"properties":{"ignored":{"type":"string"}}}`)}
	fields := declaredGatewayFields(endpoint)
	if strings.Join(fields, ",") != "sku,qty" && (len(fields) != 2 || fields[0] != "sku" && fields[1] != "sku") {
		if len(fields) != 2 {
			t.Fatalf("fields = %v", fields)
		}
	}
}
