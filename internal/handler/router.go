package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"gin-clean-starter/internal/handler/api"
	"gin-clean-starter/internal/handler/middleware"
	"gin-clean-starter/internal/pkg/config"
)

func NewRouter(engine *gin.Engine, cfg config.Config, authHandler *api.AuthHandler, authMiddleware *middleware.AuthMiddleware) {
	setupMiddleware(engine, cfg)
	setupRoutes(engine, authHandler, authMiddleware)
}

func setupMiddleware(engine *gin.Engine, cfg config.Config) {
	engine.Use(middleware.NewCORSMiddleware(cfg.CORS))
	engine.Use(middleware.LoggingMiddleware(nil, cfg.Log))
	engine.Use(gin.Recovery())
}

func setupRoutes(engine *gin.Engine, authHandler *api.AuthHandler, authMiddleware *middleware.AuthMiddleware) {
	engine.GET("/health", healthCheck)

	if gin.Mode() == gin.DebugMode {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	apiGroup := engine.Group("/api")
	{
		auth := apiGroup.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)

			authRequired := auth.Group("")
			authRequired.Use(authMiddleware.RequireAuth())
			{
				authRequired.POST("/logout", authHandler.Logout)
				authRequired.GET("/me", authHandler.Me)
			}
		}
	}
}

// @Summary Health check
// @Description Check if the service is healthy
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Service is healthy",
	})
}
