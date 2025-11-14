// internal/routes/router.go
package routes

import (
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/app"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	customMiddleware "github.com/diagnosis/luxsuv-api-v2/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func SetRouter(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	// --- Core middleware (applies to all routes)
	corsConfig := customMiddleware.DefaultCORSConfig()
	r.Use(customMiddleware.CORS(corsConfig))

	// Default global rate limit (general API traffic)
	globalLimiter := customMiddleware.NewRateLimiter(60, time.Minute)
	r.Use(customMiddleware.RateLimit(globalLimiter))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logger.HandlerLogger)
	r.Use(middleware.Recoverer)

	// --- Public
	r.Get("/healthz", app.HealthHandler.HandleHealth)

	r.Route("/api/v1", func(api chi.Router) {
		// Tighter limits for auth-sensitive endpoints
		authLimiter := customMiddleware.NewRateLimiter(30, time.Minute)

		// --- Auth (public, but rate-limited more strictly)
		api.Group(func(auth chi.Router) {
			auth.Use(customMiddleware.RateLimit(authLimiter))
			auth.Post("/auth/login", app.UserHandler.HandleLogin)
			auth.Post("/auth/register/rider", app.UserHandler.HandleRiderRegister)
			auth.Post("/auth/register/driver", app.UserHandler.HandleDriverRegister)
			auth.Get("/auth/verify", app.UserHandler.HandleVerifyEmail)
			auth.Post("/auth/forgot", app.UserHandler.HandleForgotPassword)
			// auth.Post("/auth/reset", app.UserHandler.HandleResetPassword) // when ready
		})

		// --- Protected (any signed-in user)
		api.Group(func(protected chi.Router) {
			protected.Use(customMiddleware.RequireJWT(app.Signer))
			// protected.Get("/me", app.UserHandler.HandleMe)
		})

		// --- Admin area (admin OR super_admin) — for future admin UI
		api.Group(func(adminOnly chi.Router) {
			adminOnly.Use(customMiddleware.RequireJWT(app.Signer))
			adminOnly.Use(customMiddleware.RequireRole("admin", "super_admin"))
			adminOnly.Get("/admin/driver-applications/{id}", app.AdminHandler.HandleGetDriverApplicationByApplicationID)
			adminOnly.Get("/admin/driver-applications/{userID}", app.AdminHandler.HandleGetDriverApplicationByUserID)
			adminOnly.Patch("/admin/driver-applications/{id}", app.AdminHandler.HandleReviewDriverApplication)
			// adminOnly.Get("/drivers", app.AdminHandler.ListDrivers)
		})

		// --- Super-admin only (role management)
		api.Group(func(sa chi.Router) {
			sa.Use(customMiddleware.RequireJWT(app.Signer))
			sa.Use(customMiddleware.RequireRole("super_admin"))

			// Create a user with a specific role
			sa.Post("/admin/users", app.AdminHandler.HandleCreateUserWithRole)

			// Update a user's role
			sa.Patch("/admin/users/{id}/role", app.AdminHandler.HandleSetUserRole)
		})

		// --- Driver-only (example bucket)
		api.Group(func(driverOnly chi.Router) {
			driverOnly.Use(customMiddleware.RequireJWT(app.Signer))
			driverOnly.Use(customMiddleware.RequireRole("driver"))
			// driverOnly.Get("/driver/app", app.DriverHandler.GetApplication)
		})
	})

	return r
}
