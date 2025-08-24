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
	"sync/atomic"
	"testing"
	"time"

	"gin-clean-starter/cmd/bootstrap"
	"gin-clean-starter/cmd/bootstrap/components"
	"gin-clean-starter/internal/infra/db"
	"gin-clean-starter/internal/pkg/config"

	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

var (
	postgresContainerOnce sync.Once
	postgresTestContainer testcontainers.Container

	testUser     = "test"
	testPassword = "testpass"
)

type TxPool struct {
	tx pgx.Tx
}

func NewTxPool(tx pgx.Tx) *TxPool {
	return &TxPool{tx: tx}
}

func (tp *TxPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tp.tx.Exec(ctx, sql, args...)
}

func (tp *TxPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return tp.tx.Query(ctx, sql, args...)
}

func (tp *TxPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tp.tx.QueryRow(ctx, sql, args...)
}

func (tp *TxPool) Begin(ctx context.Context) (pgx.Tx, error) {
	return tp.tx, nil // Return the same transaction
}

func (tp *TxPool) Close() {
	// No-op for transactions
}

func (tp *TxPool) Ping(ctx context.Context) error {
	return nil // Always healthy for transactions
}

func (tp *TxPool) Config() *pgxpool.Config {
	return nil // Not applicable for transactions
}

func (tp *TxPool) Stat() *pgxpool.Stat {
	return nil // Not applicable for transactions
}

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

	router, cfg := buildE2EApp(pool, dbConfig)
	require.NotNil(t, router, "Routerのセットアップに失敗")

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
			slog.Warn("データベース作成を再試行中", "attempt", attempts+1, "error", createErr, "retry_wait", waitTime)
		} else {
			slog.Warn("データベース作成を再試行中", "attempt", attempts+1, "error", createErr)
		}
	}
	require.NoError(t, createErr, "テスト用データベースの作成に失敗")

	// クリーンアップ（コンテナ自体は自動で消えるが、異常終了時も考慮）
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		cleanupPool, err := pgxpool.New(cleanupCtx, adminDSN)
		if err != nil {
			slog.Warn("クリーンアップ用のデータベース接続に失敗しました", "database", dbName, "error", err)
			return
		}
		defer cleanupPool.Close()

		_, err = cleanupPool.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+dbName)
		if err != nil {
			slog.Warn("テストデータベースの削除に失敗しました", "database", dbName, "error", err)
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

	if err := seedReferenceData(pool); err != nil {
		require.NoError(t, err, "参照データの投入に失敗")
	}

	if gin.Mode() != gin.TestMode {
		slog.Info("データベースの準備が完了しました", "postgres_schema", dbName)
	}
	return pool, dbConfig
}

func seedReferenceData(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// 初期データの投入
	_, err := pool.Exec(ctx, `
		INSERT INTO companies (id, name) VALUES 
		    (gen_random_uuid(), 'Default Company'),
		    (gen_random_uuid(), 'Test Company')
		ON CONFLICT (name) DO NOTHING;
	`)
	if err != nil {
		return err
	}

	// ユーザーは各テストで作成するため、ここでは投入しない
	return nil
}

func applyMigrations(t *testing.T, dbConfig config.DBConfig) error {
	t.Helper()

	ctx := context.Background()
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
// ------------------------------------------------------------
func buildE2EApp(pool *pgxpool.Pool, dbConfig config.DBConfig) (*gin.Engine, config.Config) {
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
	defer func() {
		if err := app.Stop(ctx); err != nil {
			slog.Warn("fxアプリケーションの停止に失敗しました", "error", err)
		}
	}()

	if router == nil {
		panic("fxアプリケーションの起動に失敗しました")
	}

	return router, cfg
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
		Reuse:            true,
	})
}

// ------------------------------------------------------------
// PostgreSQLコンテナを一度だけ起動／再利用
// ------------------------------------------------------------
func startPostgreSQLContainerOnce(t *testing.T) {
	postgresContainerOnce.Do(func() {
		// testcontainers Docker-in-Docker configuration
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
		_ = os.Setenv("RYUK_DISABLED", "true")
		_ = os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")

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
	Router   *gin.Engine
	DB       *TxPool // 各テストで使うトランザクションラッパー
	Config   config.Config
	baseDB   *pgxpool.Pool // Tx作成元のDB接続
	baseTx   pgx.Tx        // ベーストランザクション
	app      *fx.App
	baseOpts fx.Option
}

func (s *SharedSuite) GetBaseDB() *pgxpool.Pool {
	return s.baseDB
}

func (s *SharedSuite) SetupSharedSuite(t *testing.T) {
	baseDB, router, cfg := setupE2EEnvironment(t)
	s.baseDB = baseDB
	s.Router = router
	s.Config = cfg
	require.NotNil(t, baseDB, "DBのセットアップに失敗")
	require.NotEmpty(t, s.Config, "Configの取得に失敗")
	require.NotNil(t, s.Router, "Routerのセットアップに失敗")

	ctx := context.Background()
	baseTx, err := s.baseDB.Begin(ctx)
	require.NoError(t, err)
	s.baseTx = baseTx

	t.Cleanup(func() {
		_ = s.baseTx.Rollback(context.Background())
	})

	configProvider := func() config.Config { return s.Config }

	s.baseOpts = fx.Options(
		bootstrap.LoggerModule,
		bootstrap.JWTModule,
		components.RepositoryModule,
		components.UseCaseModule,
		components.HandlerModule,
		fx.Module("testConfig",
			fx.Provide(configProvider),
		),
	)
}

func (s *SharedSuite) SetupSuite() {
	s.SetupSharedSuite(s.T())
}

func (s *SharedSuite) SetupTest() {
	s.DB = NewTxPool(s.baseTx)

	var router *gin.Engine
	testApp := fx.New(
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		s.baseOpts,
		fx.Provide(func() *gin.Engine { return gin.New() }),
		fx.Provide(func() *pgxpool.Pool { return s.baseDB }),
		fx.Decorate(func(*pgxpool.Pool) *pgxpool.Pool {
			// 実際のTxPoolの実装は複雑なため、基本DBプールを使用
			// 理想的にはsqlc.DBTXラッパーを実装すべき
			return s.baseDB
		}),
		fx.Populate(&router),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(s.T(), testApp.Start(ctx))
	s.Router = router
	s.app = testApp

	s.T().Cleanup(func() {
		if s.app != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.app.Stop(ctx); err != nil {
				slog.Warn("fxアプリケーションの停止に失敗しました", "error", err)
			}
			s.app = nil
		}
	})
}

var spSeq uint64

func (s *SharedSuite) SetupSubTest() {
	sp := fmt.Sprintf("sp_%d", atomic.AddUint64(&spSeq, 1))
	ctx := context.Background()

	_, err := s.baseTx.Exec(ctx, "SAVEPOINT "+sp)
	require.NoError(s.T(), err)

	s.T().Cleanup(func() {
		_, _ = s.baseTx.Exec(context.Background(), "ROLLBACK TO SAVEPOINT "+sp)
		_, _ = s.baseTx.Exec(context.Background(), "RELEASE SAVEPOINT "+sp)
	})
}
