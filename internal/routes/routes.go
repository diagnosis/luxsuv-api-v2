package routes

import (
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/app"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	customMiddleware "github.com/diagnosis/luxsuv-api-v2/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func SetRouter(app *app.Applicaiton) *chi.Mux {
	r := chi.NewRouter()

	corsConfig := customMiddleware.DefaultCORSConfig()
	r.Use(customMiddleware.CORS(corsConfig))

	rateLimiter := customMiddleware.NewRateLimiter(100, time.Minute)
	r.Use(customMiddleware.RateLimit(rateLimiter))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logger.HandlerLogger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", app.HealthHandler.HandleHealth)

	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/login", app.UserHandler.HandleLogin)

		api.Group(func(protected chi.Router) {
			protected.Use(customMiddleware.RequireJWT(app.Signer))
		})

		api.Group(func(adminOnly chi.Router) {
			adminOnly.Use(customMiddleware.RequireJWT(app.Signer))
			adminOnly.Use(customMiddleware.RequireRole("admin"))
		})

		api.Group(func(driverOnly chi.Router) {
			driverOnly.Use(customMiddleware.RequireJWT(app.Signer))
			driverOnly.Use(customMiddleware.RequireRole("driver"))
		})
	})

	return r
}
