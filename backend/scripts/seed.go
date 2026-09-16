package main

// seed.go - Manual RBAC role and permission bootstrap tool.
// It is not called by server startup and does not create NIMNear product records.
// Run explicitly with: go run scripts/seed.go

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Bootstrapping RBAC roles and permissions...")

	// Seed roles
	if err := seedRoles(ctx, db); err != nil {
		log.Fatalf("Failed to seed roles: %v", err)
	}

	fmt.Println("RBAC bootstrap completed successfully.")
}

type sqlExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func seedRoles(ctx context.Context, db sqlExecutor) error {
	roles := []struct {
		name        string
		description string
		permissions []string
	}{
		{
			name:        "admin",
			description: "Full system administrator",
			permissions: []string{"*"},
		},
		{
			name:        "org_admin",
			description: "Organization administrator",
			permissions: []string{"org:*", "app:*", "user:*"},
		},
		{
			name:        "app_admin",
			description: "Application administrator",
			permissions: []string{"app:*", "endpoint:*"},
		},
		{
			name:        "developer",
			description: "Developer with read/write access",
			permissions: []string{"endpoint:read", "endpoint:write"},
		},
		{
			name:        "viewer",
			description: "Read-only access",
			permissions: []string{"*:read"},
		},
	}

	for _, r := range roles {
		roleID := uuid.New()
		_, err := db.Exec(ctx, `
			INSERT INTO roles (id, name, description, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			ON CONFLICT (name) DO NOTHING
		`, roleID, r.name, r.description)
		if err != nil {
			return fmt.Errorf("insert role %s: %w", r.name, err)
		}

		// Insert permissions
		for _, perm := range r.permissions {
			_, err := db.Exec(ctx, `
				INSERT INTO role_permissions (role_id, permission, created_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT (role_id, permission) DO NOTHING
			`, roleID, perm)
			if err != nil {
				return fmt.Errorf("insert permission %s for role %s: %w", perm, r.name, err)
			}
		}

		fmt.Printf("  bootstrapped role: %s\n", r.name)
	}

	return nil
}
