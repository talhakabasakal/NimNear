package gateway

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/masterfabric-go/masterfabric/internal/domain/apimanagement/model"
)

var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var deniedGatewayTables = map[string]struct{}{
	"users":                       {},
	"user_nimiq_identities":       {},
	"auth_challenges":             {},
	"event_purchases":             {},
	"payment_requests":            {},
	"consumed_nimiq_transactions": {},
	"audit_logs":                  {},
	"goose_db_version":            {},
	"role_permissions":            {},
	"user_roles":                  {},
	"organization_users":          {},
	"app_api_keys":                {},
	"app_endpoints":               {},
	"app_endpoint_policies":       {},
	"places":                      {},
	"events":                      {},
	"event_participants":          {},
	"calendars":                   {},
	"calendar_followers":          {},
	"profiles":                    {},
	"organizations":               {},
	"apps":                        {},
	"workspaces":                  {},
	"roles":                       {},
}

var reservedGatewayColumns = map[string]struct{}{
	"id":               {},
	"organization_id":  {},
	"app_id":           {},
	"created_at":       {},
	"updated_at":       {},
	"password":         {},
	"password_hash":    {},
	"api_key":          {},
	"secret":           {},
	"token":            {},
	"transaction_hash": {},
}

// quoteSQLIdentifier accepts only a safe unquoted identifier grammar, then
// applies PostgreSQL identifier quoting. Schema qualification is rejected.
func quoteSQLIdentifier(name string) (string, error) {
	if err := validateSQLIdentifier(name); err != nil {
		return "", err
	}
	return pgx.Identifier{name}.Sanitize(), nil
}

func validateSQLIdentifier(name string) error {
	if name == "" || !sqlIdentifierPattern.MatchString(name) {
		return fmt.Errorf("invalid SQL identifier")
	}
	return nil
}

func authorizeGatewayTable(name string, allowlist map[string]struct{}) error {
	if err := validateSQLIdentifier(name); err != nil {
		return err
	}
	lower := strings.ToLower(name)
	if _, denied := deniedGatewayTables[lower]; denied {
		return fmt.Errorf("table is not allowed")
	}
	if len(allowlist) == 0 {
		return fmt.Errorf("table is not allowed")
	}
	if _, ok := allowlist[lower]; !ok {
		return fmt.Errorf("table is not allowed")
	}
	return nil
}

func allowlistSet(tables []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		name := strings.ToLower(strings.TrimSpace(table))
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func declaredGatewayFields(endpoint *model.Endpoint) []string {
	if endpoint == nil || len(endpoint.Schema) == 0 {
		return nil
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(endpoint.Schema, &schema); err != nil {
		return nil
	}
	if raw, ok := schema["allowed_fields"].([]interface{}); ok {
		fields := make([]string, 0, len(raw))
		for _, item := range raw {
			if name, ok := item.(string); ok {
				fields = append(fields, name)
			}
		}
		return fields
	}
	properties, _ := schema["properties"].(map[string]interface{})
	fields := make([]string, 0, len(properties))
	for name := range properties {
		fields = append(fields, name)
	}
	return fields
}

func authorizedColumns(input map[string]interface{}, declared []string) ([]string, []interface{}, error) {
	allowed := make(map[string]struct{}, len(declared))
	for _, name := range declared {
		if err := validateSQLIdentifier(name); err != nil {
			return nil, nil, err
		}
		lower := strings.ToLower(name)
		if _, reserved := reservedGatewayColumns[lower]; reserved {
			continue
		}
		allowed[lower] = struct{}{}
	}
	columns := make([]string, 0, len(input))
	values := make([]interface{}, 0, len(input))
	for key, value := range input {
		if err := validateSQLIdentifier(key); err != nil {
			return nil, nil, err
		}
		lower := strings.ToLower(key)
		if _, reserved := reservedGatewayColumns[lower]; reserved {
			return nil, nil, fmt.Errorf("column is not allowed")
		}
		if _, ok := allowed[lower]; !ok {
			return nil, nil, fmt.Errorf("column is not allowed")
		}
		quoted, err := quoteSQLIdentifier(key)
		if err != nil {
			return nil, nil, err
		}
		columns = append(columns, quoted)
		values = append(values, value)
	}
	return columns, values, nil
}
