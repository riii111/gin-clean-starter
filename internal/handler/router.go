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

type route struct {
	Method  string
	Path    string
	Handler gin.HandlerFunc
	Mw      []gin.HandlerFunc
}

func NewRouter(engine *gin.Engine, cfg config.Config, authHandler *api.AuthHandler, reservationHandler *api.ReservationHandler, authMiddleware *middleware.AuthMiddleware) {
	setupMiddleware(engine, cfg)
	setupRoutes(engine, authHandler, reservationHandler, authMiddleware)
}

func setupMiddleware(engine *gin.Engine, cfg config.Config) {
	// Recovery must be first (outermost) to catch panics from all other middleware
	engine.Use(middleware.CustomRecovery())
	engine.Use(middleware.NewCORSMiddleware(cfg.CORS))
	engine.Use(middleware.LoggingMiddleware(nil, cfg.Log))
	engine.Use(middleware.ErrorHandler())
}

func setupRoutes(engine *gin.Engine, authHandler *api.AuthHandler, reservationHandler *api.ReservationHandler, authMiddleware *middleware.AuthMiddleware) {
	engine.GET("/health", healthCheck)

	if gin.Mode() == gin.DebugMode {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	apiGroup := engine.Group("/api")
	{
		auth := apiGroup.Group("/auth")
		{
			addRoutes(auth, []route{
				{Method: http.MethodPost, Path: "/login", Handler: authHandler.Login},
				{Method: http.MethodPost, Path: "/refresh", Handler: authHandler.Refresh},
			})

			authRequired := auth.Group("")
			authRequired.Use(authMiddleware.RequireAuth())
			addRoutes(authRequired, []route{
				{Method: http.MethodPost, Path: "/logout", Handler: authHandler.Logout},
				{Method: http.MethodGet, Path: "/me", Handler: authHandler.Me},
			})
		}

		reservations := apiGroup.Group("/reservations")
		reservations.Use(authMiddleware.RequireAuth())
		{
			addRoutes(reservations, []route{
				{Method: http.MethodPost, Path: "", Handler: reservationHandler.CreateReservation},
				{Method: http.MethodGet, Path: "", Handler: reservationHandler.GetUserReservations},
				{Method: http.MethodGet, Path: "/:id", Handler: reservationHandler.GetReservation},
			})
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

func addRoutes(g *gin.RouterGroup, rs []route) {
	for _, r := range rs {
		h := r.Handler
		if len(r.Mw) > 0 {
			h = chainHandlers(append(r.Mw, r.Handler)...)
		}
		switch r.Method {
		case http.MethodGet:
			g.GET(r.Path, h)
		case http.MethodPost:
			g.POST(r.Path, h)
		case http.MethodPut:
			g.PUT(r.Path, h)
		case http.MethodPatch:
			g.PATCH(r.Path, h)
		case http.MethodDelete:
			g.DELETE(r.Path, h)
		default:
			g.Any(r.Path, h)
		}
	}
}

func chainHandlers(hs ...gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, h := range hs {
			h(c)
			if c.IsAborted() {
				return
			}
		}
	}
}
