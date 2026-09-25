package router

import (
	"log/slog"
	"net/http"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/config"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/handler"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/middleware"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/repository"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	vesselCallRepository := repository.NewVesselCallRepository(db)
	mooringPlanRepository := repository.NewMooringPlanRepository(db)
	weatherWindowRepository := repository.NewWeatherWindowRepository(db)
	safetyClearanceRepository := repository.NewSafetyClearanceRepository(db)
	vesselCallService := service.NewVesselCallService(vesselCallRepository, securityService)
	mooringPlanService := service.NewMooringPlanService(mooringPlanRepository, securityService)
	weatherWindowService := service.NewWeatherWindowService(weatherWindowRepository, safetyClearanceRepository, securityService)
	safetyClearanceService := service.NewSafetyClearanceService(safetyClearanceRepository, weatherWindowRepository, securityService)
	vesselCallHandler := handler.NewVesselCallHandler(vesselCallService)
	mooringPlanHandler := handler.NewMooringPlanHandler(mooringPlanService)
	weatherWindowHandler := handler.NewWeatherWindowHandler(weatherWindowService)
	safetyClearanceHandler := handler.NewSafetyClearanceHandler(safetyClearanceService)
	systemHandler := handler.NewSystemHandler(securityService, vesselCallService, mooringPlanService, weatherWindowService, safetyClearanceService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	engine.POST("/api/auth/login", systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/audits", middleware.RequireMinimumRole("reviewer"), systemHandler.Audits)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", middleware.RequireMinimumRole("reviewer"), systemHandler.Runtime)
	api.GET("/audit-summary", middleware.RequireMinimumRole("reviewer"), systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", middleware.RequireMinimumRole("reviewer"), systemHandler.EntityHistory)
	vesselCallHandler.Register(api)
	mooringPlanHandler.Register(api)
	weatherWindowHandler.Register(api)
	safetyClearanceHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}
