//go:build unit || e2e

package dbtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func CreateTestUser(t *testing.T, db DBLike, email, role string) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	var companyID uuid.UUID

	ctx := context.Background()
	err := db.QueryRow(ctx, "SELECT id FROM companies WHERE name = 'Default Company' LIMIT 1").Scan(&companyID)
	require.NoError(t, err)

	passwordHash := "$2a$12$uhAjVE9f92IGYv3E25pJNetg.27lVt0p7jmLWjqjmhOg92ldPS0A."
	tag, err := db.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, company_id, is_active) VALUES ($1, $2, $3, $4, $5, true) ON CONFLICT (email) WHERE is_active = true DO NOTHING",
		userID, email, passwordHash, role, companyID)
	require.NoError(t, err)

	if tag.RowsAffected() == 0 {
		_ = db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND is_active = true", email).Scan(&userID)
	}

	return userID
}

func CreateTestCompany(t *testing.T, db DBLike, name string) uuid.UUID {
	t.Helper()

	companyID := uuid.New()
	ctx := context.Background()

	tag, err := db.Exec(ctx, "INSERT INTO companies (id, name) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING", companyID, name)
	require.NoError(t, err)

	if tag.RowsAffected() == 0 {
		_ = db.QueryRow(ctx, "SELECT id FROM companies WHERE name = $1", name).Scan(&companyID)
	}

	return companyID
}

// inserts basic reference data needed by tests
func SeedReferenceData(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Insert companies
	_, err := pool.Exec(ctx, `
		INSERT INTO companies (id, name) VALUES 
		    (gen_random_uuid(), 'Default Company'),
		    (gen_random_uuid(), 'Test Company')
		ON CONFLICT (name) DO NOTHING;
	`)
	if err != nil {
		return err
	}

	return nil
}

var (
	buildTruncateOnce sync.Once
	truncateSQL       atomic.Value // string
)

// truncates all tables and reseeds reference data
func ResetDB(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	buildTruncateOnce.Do(func() {
		rows, err := pool.Query(ctx, `
		  SELECT 'public.' || quote_ident(tablename)
		  FROM pg_tables
		  WHERE schemaname = 'public'
		    AND tablename NOT IN ('schema_migrations')`)
		if err != nil {
			truncateSQL.Store("")
			return
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				truncateSQL.Store("")
				return
			}
			tables = append(tables, t)
		}
		if rows.Err() != nil {
			truncateSQL.Store("")
			return
		}
		if len(tables) == 0 {
			truncateSQL.Store("SELECT 1")
			return
		}
		truncateSQL.Store("TRUNCATE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE;")
	})
	sqlAny := truncateSQL.Load()
	if sqlAny == nil || sqlAny.(string) == "" {
		return fmt.Errorf("failed to build TRUNCATE SQL")
	}
	if _, err := pool.Exec(ctx, sqlAny.(string)); err != nil {
		return err
	}

	return SeedReferenceData(pool)
}
