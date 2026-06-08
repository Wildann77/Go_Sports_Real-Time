package internal

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/core/middleware"
	"sports-dashboard/internal/core/security"
	apiKeyHandlers "sports-dashboard/internal/features/apikeys/handlers"
	apiKeyRepositories "sports-dashboard/internal/features/apikeys/repositories"
	apiKeyRouters "sports-dashboard/internal/features/apikeys/routers"
	apiKeySchemas "sports-dashboard/internal/features/apikeys/schemas"
	apiKeyServices "sports-dashboard/internal/features/apikeys/services"
	authHandlers "sports-dashboard/internal/features/auth/handlers"
	authRepositories "sports-dashboard/internal/features/auth/repositories"
	authRouters "sports-dashboard/internal/features/auth/routers"
	authServices "sports-dashboard/internal/features/auth/services"
	commentaryHandlers "sports-dashboard/internal/features/commentary/handlers"
	commentaryRepositories "sports-dashboard/internal/features/commentary/repositories"
	commentaryRouters "sports-dashboard/internal/features/commentary/routers"
	commentaryServices "sports-dashboard/internal/features/commentary/services"
	healthHandlers "sports-dashboard/internal/features/health/handlers"
	healthRouters "sports-dashboard/internal/features/health/routers"
	healthServices "sports-dashboard/internal/features/health/services"
	matchHandlers "sports-dashboard/internal/features/matches/handlers"
	matchRepositories "sports-dashboard/internal/features/matches/repositories"
	matchRouters "sports-dashboard/internal/features/matches/routers"
	matchServices "sports-dashboard/internal/features/matches/services"
	wsHandlers "sports-dashboard/internal/features/realtime/handlers"
	"sports-dashboard/internal/features/realtime/hub"
	wsRouters "sports-dashboard/internal/features/realtime/routers"
)

func SetupRouter(appCtx context.Context, cfg *config.Config, db *gorm.DB) (*gin.Engine, func()) {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		security.RegisterCustomValidators(v)
	}

	router := gin.New()
	timeoutPolicy := coreDatabase.NewTimeoutPolicy(cfg)
	rateLimiter := middleware.NewIPRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	rateLimiter.Start(appCtx)

	router.Use(middleware.Logging())
	router.Use(middleware.Recover())
	router.Use(middleware.CORS(cfg.AllowedOrigins))
	router.Use(rateLimiter.Middleware())
	router.Use(middleware.BodyLimit(1024 * 1024 * 5))
	router.Use(exceptions.ErrorHandlerMiddleware())

	wsHub := hub.NewHub()
	wsHub.Start(appCtx)

	matchRepo := matchRepositories.NewMatchRepository(db, timeoutPolicy)
	commentaryRepo := commentaryRepositories.NewCommentaryRepository(db, timeoutPolicy)
	authRepo := authRepositories.NewAuthRepository(db, timeoutPolicy)
	apiKeyRepo := apiKeyRepositories.NewAPIKeyRepository(db, timeoutPolicy)

	matchSvc := matchServices.NewMatchService(matchRepo)
	commentarySvc := commentaryServices.NewCommentaryService(commentaryRepo, matchRepo, wsHub, db, timeoutPolicy)
	authSvc := authServices.NewAuthService(authRepo, db, cfg, timeoutPolicy)
	apiKeySvc := apiKeyServices.NewAPIKeyService(apiKeyRepo, cfg)

	healthSvc := healthServices.NewHealthService(healthServices.NewGormDBProvider(db), cfg.DBQueryTimeout())
	healthHandler := healthHandlers.NewHealthHandler(healthSvc)
	healthRouters.SetupHealthRouter(router, healthHandler)

	v1 := router.Group("/api/v1")
	{
		authHandler := authHandlers.NewAuthHandler(authSvc, cfg)
		authRouters.SetupAuthRouter(v1, authHandler, middleware.AuthRequired(authSvc))

		apiKeyHandler := apiKeyHandlers.NewAPIKeyHandler(apiKeySvc)
		apiKeyRouters.SetupAPIKeyRouter(v1, apiKeyHandler, middleware.AuthRequired(authSvc))

		matchHandler := matchHandlers.NewMatchHandler(matchSvc)
		matchRouters.SetupMatchRouter(v1, matchHandler, middleware.RequireJWTOrAPIKey(authSvc, apiKeySvc, apiKeySchemas.ScopeMatchesWrite))

		commentaryHandler := commentaryHandlers.NewCommentaryHandler(commentarySvc)
		commentaryRouters.SetupCommentaryRouter(v1, commentaryHandler, middleware.RequireJWTOrAPIKey(authSvc, apiKeySvc, apiKeySchemas.ScopeCommentaryWrite))
	}

	wsHandler := wsHandlers.NewWebSocketHandler(wsHub, cfg.AllowedOrigins, cfg.WsMaxPayloadBytes)
	wsRouters.SetupWebSocketRouter(router, wsHandler)

	cleanup := func() {
		wsHub.Stop()
	}

	return router, cleanup
}
