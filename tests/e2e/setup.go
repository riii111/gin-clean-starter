//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gin-clean-starter/cmd/bootstrap"
	"gin-clean-starter/cmd/bootstrap/components"
	"gin-clean-starter/internal/infra/db"
	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/tests/common/dbtest"

	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

var (
	postgresContainerOnce sync.Once
	postgresTestContainer testcontainers.Container

	testUser     = "test"
	testPassword = "testpass"
)

type ContainerInfo struct {
	Host string
	Port nat.Port
}

// ------------------------------------------------------------
// E2E Environment Setup
// ------------------------------------------------------------
func setupE2EEnvironment(t *testing.T) (*pgxpool.Pool, *gin.Engine, config.Config) {
	postgresInfo := startContainers(t)

	pool, dbConfig := prepareDatabase(t, postgresInfo)

	router, cfg, app := buildE2EApp(pool, dbConfig)
	require.NotNil(t, router, "Failed to setup router")

	// Register cleanup for the fx app
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Stop(ctx); err != nil {
			slog.Warn("Failed to stop fx application", "error", err.Error())
		}
	})

	slog.Info("E2E environment ready",
		"postgres_host", postgresInfo.Host,
		"postgres_port", postgresInfo.Port.Port())

	return pool, router, cfg
}

// ------------------------------------------------------------
// Container Startup
// ------------------------------------------------------------
func startContainers(t *testing.T) ContainerInfo {
	gin.SetMode(gin.TestMode)
	startPostgreSQLContainerOnce(t)

	postgresInfo, err := getContainerHostPort(postgresTestContainer, "5432/tcp")
	require.NoError(t, err, "Failed to get PostgreSQL container info")

	return postgresInfo
}

// ------------------------------------------------------------
// Database Preparation
// ------------------------------------------------------------
func prepareDatabase(t *testing.T, postgresInfo ContainerInfo) (*pgxpool.Pool, config.DBConfig) {
	// Generate unique schema name per process
	dbName := "testdb_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable",
		testUser, testPassword, postgresInfo.Host, postgresInfo.Port.Port())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, adminDSN)
	require.NoError(t, err, "Failed to connect as admin")
	defer adminPool.Close()

	// Create database with retry mechanism
	var createErr error
	for attempts := range 5 {
		var waitTime time.Duration
		if attempts > 0 {
			// Exponential backoff
			waitTime = time.Duration(500+attempts*500) * time.Millisecond
			waitTime = min(waitTime, 3*time.Second)
			time.Sleep(waitTime)
		}
		_, createErr = adminPool.Exec(ctx, "CREATE DATABASE "+dbName)
		if createErr == nil {
			break
		}
		if attempts > 0 {
			slog.Warn("Retrying database creation", "attempt", attempts+1, "error", createErr.Error(), "retry_wait", waitTime)
		} else {
			slog.Warn("Retrying database creation", "attempt", attempts+1, "error", createErr.Error())
		}
	}
	require.NoError(t, createErr, "Failed to create test database")

	// Cleanup (container auto-removes, but handle abnormal exits)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		cleanupPool, err := pgxpool.New(cleanupCtx, adminDSN)
		if err != nil {
			slog.Warn("Failed to connect for cleanup", "database", dbName, "error", err.Error())
			return
		}
		defer cleanupPool.Close()

		_, err = cleanupPool.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+dbName)
		if err != nil {
			slog.Warn("Failed to drop test database", "database", dbName, "error", err.Error())
		}
	})

	dbConfig := config.DBConfig{
		Host:     postgresInfo.Host,
		Port:     postgresInfo.Port.Port(),
		User:     testUser,
		Password: testPassword,
		DBName:   dbName,
		SSLMode:  "disable",
		TimeZone: "Asia/Tokyo",
	}

	pool, _, err := db.Connect(dbConfig)
	require.NoError(t, err, "Failed to connect to database")
	require.NotNil(t, pool, "Database connection is nil")

	err = applyMigrations(t, dbConfig)
	require.NoError(t, err, "Failed to apply database migrations")

	if err := dbtest.SeedReferenceData(pool); err != nil {
		require.NoError(t, err, "Failed to seed reference data")
	}

	if gin.Mode() != gin.TestMode {
		slog.Info("Database setup complete", "postgres_schema", dbName)
	}
	return pool, dbConfig
}

func applyMigrations(t *testing.T, dbConfig config.DBConfig) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, _, err := db.Connect(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	migrationFiles := []string{
		"migrations/001_initial_schema.sql",
		"migrations/002_review_schema.sql",
	}

	for _, file := range migrationFiles {
		// Resolve migration file path relative to possible working dirs (package dirs during `go test`).
		var (
			sqlContent []byte
			readErr    error
		)
		candidates := []string{
			file, // repo root
			filepath.Join("..", file),
			filepath.Join("..", "..", file),
			filepath.Join("..", "..", "..", file),
		}
		for _, cand := range candidates {
			sqlContent, readErr = os.ReadFile(cand)
			if readErr == nil {
				file = cand
				break
			}
		}
		if readErr != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, readErr)
		}

		_, err = pool.Exec(ctx, string(sqlContent))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}

		slog.Info("Migration completed", "file", file)
	}

	return nil
}

// ------------------------------------------------------------
// E2E Application Builder
// Returns router, config, and fx.App for proper lifecycle management
// ------------------------------------------------------------
func buildE2EApp(pool *pgxpool.Pool, dbConfig config.DBConfig) (*gin.Engine, config.Config, *fx.App) {
	var router *gin.Engine
	var cfg config.Config

	testDBModule := fx.Module("testdb",
		fx.Provide(func() *pgxpool.Pool { return pool }),
	)

	testConfigModule := fx.Module("testconfig",
		fx.Provide(func() config.Config {
			return createTestConfig(dbConfig)
		}),
	)

	app := fx.New(
		testDBModule,
		testConfigModule,
		fx.Provide(func() *gin.Engine { return gin.New() }),
		bootstrap.LoggerModule,
		bootstrap.JWTModule,
		components.RepositoryModule,
		components.UseCaseModule,
		components.HandlerModule,

		fx.Populate(&router, &cfg),

		// Start with logging disabled
		fx.NopLogger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		panic(fmt.Sprintf("Failed to start fx app: %v", err))
	}

	if router == nil {
		panic("Failed to start fx application")
	}

	return router, cfg, app
}

func createTestConfig(dbConfig config.DBConfig) config.Config {
	testConfig := config.NewTestConfig()
	testConfig.DB = dbConfig
	return testConfig
}

// ------------------------------------------------------------
// Generic Container Startup
// ------------------------------------------------------------
func startGenericContainer(req testcontainers.ContainerRequest, timeoutSec int) (testcontainers.Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		// Reuse: true, // Removed to enable proper cleanup by ryuk
	})
}

// ------------------------------------------------------------
// Start PostgreSQL Container Once
// ------------------------------------------------------------
func startPostgreSQLContainerOnce(t *testing.T) {
	postgresContainerOnce.Do(func() {
		// testcontainers Docker-in-Docker configuration
		// Note: RYUK enabled for proper cleanup

		req := testcontainers.ContainerRequest{
			Image:        "postgres:17",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     testUser,
				"POSTGRES_PASSWORD": testPassword,
				"POSTGRES_DB":       "postgres",
			},
			Tmpfs: map[string]string{
				"/var/lib/postgresql/data": "rw,size=512m", // PostgreSQL data in RAM for I/O reduction
			},
			Cmd: []string{
				"postgres",
				"-c", "fsync=off", // Performance over durability
				"-c", "full_page_writes=off", // Disable full page writes
				"-c", "synchronous_commit=off", // Disable sync commit
				"-c", "max_wal_size=512MB", // WAL file size limit
				"-c", "checkpoint_completion_target=0.9", // Checkpoint completion target
				"-c", "wal_buffers=16MB", // WAL buffer size
				"-c", "shared_buffers=256MB", // Shared buffer size
				"-c", "max_connections=200", // Max connections
				"-c", "log_statement=none", // Disable statement logging
				"-c", "log_duration=off", // Disable duration logging
				"-c", "log_lock_waits=off", // Disable lock wait logging
				"-c", "log_checkpoints=off", // Disable checkpoint logging
				"-c", "autovacuum=on", // Enable autovacuum
				"-c", "autovacuum_max_workers=2", // Reduce vacuum workers
			},
			WaitingFor: wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable",
					testUser, testPassword, host, port.Port())
			}).WithStartupTimeout(60 * time.Second),
			Labels: map[string]string{"purpose": "e2e-tests"},
		}

		var err error
		postgresTestContainer, err = startGenericContainer(req, 180)
		require.NoError(t, err, "Failed to start PostgreSQL container")

		// Register manual cleanup (for RYUK disabled)
		t.Cleanup(func() {
			if postgresTestContainer != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := postgresTestContainer.Terminate(ctx); err != nil {
					slog.Warn("Failed to terminate PostgreSQL container", "error", err.Error())
				}
			}
		})
	})
}

// ------------------------------------------------------------
// Container Utility Functions
// ------------------------------------------------------------
func getContainerHostPort(c testcontainers.Container, port string) (ContainerInfo, error) {
	ctx := context.Background()
	mappedPort, err := c.MappedPort(ctx, nat.Port(port))
	if err != nil {
		return ContainerInfo{}, err
	}
	host, err := c.Host(ctx)
	if err != nil {
		return ContainerInfo{}, err
	}
	return ContainerInfo{Host: host, Port: mappedPort}, nil
}

// ------------------------------------------------------------
// Shared E2E Test Suite Setup
// ------------------------------------------------------------
type SharedSuite struct {
	suite.Suite
	Router *gin.Engine
	DB     *pgxpool.Pool // DB connection for each test
	Config config.Config
}

func (s *SharedSuite) SetupSharedSuite(t *testing.T) {
	db, router, cfg := setupE2EEnvironment(t)
	s.DB = db
	s.Router = router
	s.Config = cfg
	require.NotNil(t, db, "Failed to setup DB")
	require.NotEmpty(t, s.Config, "Failed to get Config")
	require.NotNil(t, s.Router, "Failed to setup Router")
}

func (s *SharedSuite) SetupSuite() {
	s.SetupSharedSuite(s.T())
}

func (s *SharedSuite) SetupTest() {
	// No additional setup needed for tests
	// Each test method can reset DB state if needed
}

func (s *SharedSuite) SetupSubTest() {
	// Reset database state using TRUNCATE + reseed approach
	err := dbtest.ResetDB(s.DB)
	require.NoError(s.T(), err, "Failed to reset database state")
}
