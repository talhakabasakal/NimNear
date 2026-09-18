package testdb

import "testing"

func TestDatabaseNameFromPostgresURL(t *testing.T) {
	name, err := databaseName("postgres://nimnear:nimnear@localhost:5432/nimnear_test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if name != "nimnear_test" {
		t.Fatalf("name = %q", name)
	}
}

func TestIsolationRejectsDevelopmentAndProductionDatabases(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		isolated string
	}{
		{name: "application db", dsn: "postgres://nimnear:nimnear@localhost:5432/nimnear?sslmode=disable", isolated: "true"},
		{name: "production-like name", dsn: "postgres://nimnear:nimnear@localhost:5432/nimnear_prod?sslmode=disable", isolated: "true"},
		{name: "missing isolated flag", dsn: "postgres://nimnear:nimnear@localhost:5432/nimnear_test?sslmode=disable", isolated: ""},
		{name: "masterfabric default", dsn: "postgres://masterfabric:masterfabric@localhost:5432/masterfabric?sslmode=disable", isolated: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := isolationError(tt.dsn, tt.isolated); err == nil {
				t.Fatal("expected isolation error")
			}
		})
	}
}

func TestIsolationAcceptsNamedTestDatabase(t *testing.T) {
	if err := isolationError("postgres://nimnear:nimnear@localhost:5432/nimnear_test?sslmode=disable", "true"); err != nil {
		t.Fatal(err)
	}
}
