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

	// --- Core middleware
	corsConfig := customMiddleware.DefaultCORSConfig()
	r.Use(customMiddleware.CORS(corsConfig))

	rateLimiter := customMiddleware.NewRateLimiter(100, time.Minute)
	r.Use(customMiddleware.RateLimit(rateLimiter))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logger.HandlerLogger)
	r.Use(middleware.Recoverer)

	// --- Public
	r.Get("/healthz", app.HealthHandler.HandleHealth)

	r.Route("/api/v1", func(api chi.Router) {

		// ---- Auth (public)
		api.Post("/auth/login", app.UserHandler.HandleLogin)

		// Registration (rider/driver)
		api.Post("/auth/register/rider", app.UserHandler.HandleRiderRegister)
		api.Post("/auth/register/driver", app.UserHandler.HandleDriverRegister)

		// Email verification callback
		// expects: /api/v1/auth/verify?token=...&purpose=rider_confirm|driver_confirm
		api.Get("/auth/verify", app.UserHandler.HandleVerifyEmail)

		// (Optional) add when you implement
		// api.Post("/auth/refresh", app.UserHandler.HandleRefresh)
		// api.Post("/auth/logout", app.UserHandler.HandleLogout)

		// ---- Protected (any authenticated user)
		api.Group(func(protected chi.Router) {
			protected.Use(customMiddleware.RequireJWT(app.Signer))

			// add user endpoints here, e.g.:
			// protected.Get("/me", app.UserHandler.HandleMe)
		})

		// ---- Admin-only (admin OR super_admin)
		api.Group(func(adminOnly chi.Router) {
			adminOnly.Use(customMiddleware.RequireJWT(app.Signer))
			// If your middleware accepts multiple roles:
			// adminOnly.Use(customMiddleware.RequireRole("admin", "super_admin"))
			// If you have a helper that takes a slice:
			//adminOnly.Use(customMiddleware.RequireRoleAny([]string{"admin", "super_admin"}))

			// admin routes:
			// adminOnly.Get("/drivers", app.AdminHandler.ListDrivers)
			// adminOnly.Patch("/drivers/{userID}/approve", app.AdminHandler.ApproveDriver)
			// adminOnly.Patch("/drivers/{userID}/reject", app.AdminHandler.RejectDriver)
		})

		// ---- Driver-only
		api.Group(func(driverOnly chi.Router) {
			driverOnly.Use(customMiddleware.RequireJWT(app.Signer))
			driverOnly.Use(customMiddleware.RequireRole("driver"))

			// driver routes:
			// driverOnly.Get("/driver/app", app.DriverHandler.GetApplication)
		})
	})

	return r
}
