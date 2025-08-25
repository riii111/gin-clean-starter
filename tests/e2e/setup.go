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
// 各テストプロセス用にセットアップ
// ------------------------------------------------------------
func setupE2EEnvironment(t *testing.T) (*pgxpool.Pool, *gin.Engine, config.Config) {
	postgresInfo := startContainers(t)

	pool, dbConfig := prepareDatabase(t, postgresInfo)

	router, cfg, app := buildE2EApp(pool, dbConfig)
	require.NotNil(t, router, "Routerのセットアップに失敗")

	// Register cleanup for the fx app
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Stop(ctx); err != nil {
			slog.Warn("fxアプリケーションの停止に失敗しました", "error", err.Error())
		}
	})

	slog.Info("E2E環境の準備が完了しました",
		"postgres_host", postgresInfo.Host,
		"postgres_port", postgresInfo.Port.Port())

	return pool, router, cfg
}

// ------------------------------------------------------------
// コンテナ起動関数
// ------------------------------------------------------------
func startContainers(t *testing.T) ContainerInfo {
	gin.SetMode(gin.TestMode)
	startPostgreSQLContainerOnce(t)

	postgresInfo, err := getContainerHostPort(postgresTestContainer, "5432/tcp")
	require.NoError(t, err, "PostgreSQLコンテナ情報の取得に失敗")

	return postgresInfo
}

// ------------------------------------------------------------
// データベース準備関数
// ------------------------------------------------------------
func prepareDatabase(t *testing.T, postgresInfo ContainerInfo) (*pgxpool.Pool, config.DBConfig) {
	// プロセス毎に違うスキーマ名を生成
	dbName := "testdb_" + strings.ReplaceAll(uuid.New().String(), "-", "") // ハイフンを除去してDB名として使用

	adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable",
		testUser, testPassword, postgresInfo.Host, postgresInfo.Port.Port())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, adminDSN)
	require.NoError(t, err, "管理者接続に失敗")
	defer adminPool.Close()

	// データベース作成をリトライ機構付きで実行
	var createErr error
	for attempts := range 5 {
		var waitTime time.Duration
		if attempts > 0 {
			// 指数バックオフ
			waitTime = time.Duration(500+attempts*500) * time.Millisecond
			waitTime = min(waitTime, 3*time.Second)
			time.Sleep(waitTime)
		}
		_, createErr = adminPool.Exec(ctx, "CREATE DATABASE "+dbName)
		if createErr == nil {
			break
		}
		if attempts > 0 {
			slog.Warn("データベース作成を再試行中", "attempt", attempts+1, "error", createErr.Error(), "retry_wait", waitTime)
		} else {
			slog.Warn("データベース作成を再試行中", "attempt", attempts+1, "error", createErr.Error())
		}
	}
	require.NoError(t, createErr, "テスト用データベースの作成に失敗")

	// クリーンアップ（コンテナ自体は自動で消えるが、異常終了時も考慮）
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		cleanupPool, err := pgxpool.New(cleanupCtx, adminDSN)
		if err != nil {
			slog.Warn("クリーンアップ用のデータベース接続に失敗しました", "database", dbName, "error", err.Error())
			return
		}
		defer cleanupPool.Close()

		_, err = cleanupPool.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+dbName)
		if err != nil {
			slog.Warn("テストデータベースの削除に失敗しました", "database", dbName, "error", err.Error())
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
	require.NoError(t, err, "データベース接続に失敗")
	require.NotNil(t, pool, "データベース接続が nil です")

	err = applyMigrations(t, dbConfig)
	require.NoError(t, err, "データベースマイグレーションに失敗")

	if err := dbtest.SeedReferenceData(pool); err != nil {
		require.NoError(t, err, "参照データの投入に失敗")
	}

	if gin.Mode() != gin.TestMode {
		slog.Info("データベースの準備が完了しました", "postgres_schema", dbName)
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

		slog.Info("マイグレーション実行完了", "file", file)
	}

	return nil
}

// ------------------------------------------------------------
// E2Eテスト用アプリケーション構築関数
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

		// ログを無効にして起動
		fx.NopLogger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		panic(fmt.Sprintf("Failed to start fx app: %v", err))
	}

	if router == nil {
		panic("fxアプリケーションの起動に失敗しました")
	}

	return router, cfg, app
}

func createTestConfig(dbConfig config.DBConfig) config.Config {
	testConfig := config.NewTestConfig()
	testConfig.DB = dbConfig
	return testConfig
}

// ------------------------------------------------------------
// コンテナ起動の共通関数
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
// PostgreSQLコンテナを一度だけ起動／再利用
// ------------------------------------------------------------
func startPostgreSQLContainerOnce(t *testing.T) {
	postgresContainerOnce.Do(func() {
		// testcontainers Docker-in-Docker configuration
		// Note: RYUK is enabled for proper cleanup in local development

		req := testcontainers.ContainerRequest{
			Image:        "postgres:17",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     testUser,
				"POSTGRES_PASSWORD": testPassword,
				"POSTGRES_DB":       "postgres",
			},
			Tmpfs: map[string]string{
				"/var/lib/postgresql/data": "rw,size=512m", // PostgreSQLデータをRAMに載せてI/O削減
			},
			Cmd: []string{
				"postgres",
				"-c", "fsync=off", // 耐久性よりパフォーマンスを優先
				"-c", "full_page_writes=off", // フルページ書き込み無効
				"-c", "synchronous_commit=off", // 同期コミット無効
				"-c", "max_wal_size=512MB", // WALファイルサイズ上限
				"-c", "checkpoint_completion_target=0.9", // チェックポイント完了目標時間
				"-c", "wal_buffers=16MB", // WALバッファ増量
				"-c", "shared_buffers=256MB", // 共有バッファ増量
				"-c", "max_connections=200", // 最大接続数
				"-c", "log_statement=none", // ログ無効化
				"-c", "log_duration=off", // 実行時間ログ無効
				"-c", "log_lock_waits=off", // ロック待ちログ無効
				"-c", "log_checkpoints=off", // チェックポイントログ無効
				"-c", "autovacuum=on", // オートバキューム有効
				"-c", "autovacuum_max_workers=2", // バキュームワーカー削減
			},
			WaitingFor: wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable",
					testUser, testPassword, host, port.Port())
			}).WithStartupTimeout(60 * time.Second),
			Name:   "postgres-e2e",
			Labels: map[string]string{"purpose": "e2e-tests"},
		}

		var err error
		postgresTestContainer, err = startGenericContainer(req, 180)
		require.NoError(t, err, "PostgreSQLコンテナの起動に失敗")

		// コンテナの手動クリーンアップを登録 (RYUK無効時用)
		t.Cleanup(func() {
			if postgresTestContainer != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := postgresTestContainer.Terminate(ctx); err != nil {
					slog.Warn("PostgreSQLコンテナの終了に失敗しました", "error", err.Error())
				}
			}
		})
	})
}

// ------------------------------------------------------------
// コンテナ関連の共通ユーティリティ関数
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
// E2Eテストスイートで共通のセットアップ
// ------------------------------------------------------------
type SharedSuite struct {
	suite.Suite
	Router *gin.Engine
	DB     *pgxpool.Pool // 各テストで使う DB 接続
	Config config.Config
}

func (s *SharedSuite) SetupSharedSuite(t *testing.T) {
	db, router, cfg := setupE2EEnvironment(t)
	s.DB = db
	s.Router = router
	s.Config = cfg
	require.NotNil(t, db, "DBのセットアップに失敗")
	require.NotEmpty(t, s.Config, "Configの取得に失敗")
	require.NotNil(t, s.Router, "Routerのセットアップに失敗")
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
