package main

import (
	"context"
	"log/slog"
	"os"

	"gin-clean-starter/cmd/bootstrap"
	"gin-clean-starter/internal/handler/middleware"
	"gin-clean-starter/internal/pkg/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func init() {
	// 設定ミスでもデバッグ情報を公開しない（フェイルセーフ）
	gin.SetMode(gin.ReleaseMode)

	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	}
}

// @title           gin-clean-starter
// @version         1.0
// @description
// @description

// @BasePath  /
// @schemes http https
// @in header
func startServer(lc fx.Lifecycle, engine *gin.Engine, cfg config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			gin.EnableJsonDecoderDisallowUnknownFields()
			listenAddr := ":" + cfg.Server.Port
			logger.Info("🚀 サーバーを起動します", "address", listenAddr, "mode", gin.Mode())
			go func() {
				if err := engine.Run(listenAddr); err != nil {
					logger.Error("サーバーの起動に失敗しました", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			logger.Info("🛑 サーバーを停止します")
			return nil
		},
	})
}

func main() {
	app := fx.New(
		bootstrap.Module,
		fx.Provide(
			func(cfg config.Config) *slog.Logger {
				logger := middleware.NewLogger(cfg.Log)
				return logger.GetSlogLogger()
			},
			func() *gin.Engine {
				return gin.New()
			},
		),
		fx.Invoke(
			startServer,
		),
	)

	if err := app.Start(context.Background()); err != nil {
		slog.Error("アプリケーションの起動に失敗しました", "error", err)
		os.Exit(1)
	}

	<-app.Done()

	if err := app.Stop(context.Background()); err != nil {
		slog.Error("アプリケーションの停止に失敗しました", "error", err)
		// Djangoと同様、Exitしない
	}

	slog.Info("アプリケーションが正常に停止しました")
}
