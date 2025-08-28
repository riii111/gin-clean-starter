package main

import (
	"context"
	"log/slog"
	"os"

	"gin-clean-starter/cmd/bootstrap"
	"gin-clean-starter/internal/pkg/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func init() {
	// Do not expose debug information even if misconfigured (fail-safe)
	gin.SetMode(gin.ReleaseMode)

	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	}
}

// @title           Gin Clean Starter
// @version         1.0
// @description     JWT Authorization header using the Bearer scheme
// @BasePath  /
// @schemes http https
// @in header      Authorization
// @name          Authorization
func startServer(lc fx.Lifecycle, engine *gin.Engine, cfg config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			gin.EnableJsonDecoderDisallowUnknownFields()
			listenAddr := ":" + cfg.Server.Port
			logger.Info("🚀 Starting server", "address", listenAddr, "mode", gin.Mode())
			go func() {
				if err := engine.Run(listenAddr); err != nil {
					logger.Error("Failed to start server", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			logger.Info("🛑 Stopping server")
			return nil
		},
	})
}

func main() {
	app := fx.New(
		bootstrap.Module,
		fx.Provide(
			func() *gin.Engine {
				return gin.New()
			},
		),
		fx.Invoke(
			startServer,
		),
	)

	if err := app.Start(context.Background()); err != nil {
		slog.Error("Failed to start application", "error", err.Error())
		os.Exit(1)
	}

	<-app.Done()

	if err := app.Stop(context.Background()); err != nil {
		slog.Error("Failed to stop application", "error", err.Error())
		// don't exit
	}

	slog.Info("Application stopped successfully")
}
