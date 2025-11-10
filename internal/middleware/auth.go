package middleware

import (
	"net/http"
	"strings"

	"github.com/diagnosis/luxsuv-api-v2/internal/apperror"
	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/secure"
)

// bearerTokenFromRequest extracts an access token from the Authorization header,
// falling back to a cookie named "access_token" (useful for SPAs).
func bearerTokenFromRequest(r *http.Request) string {
	// Authorization: Bearer <token>
	auth := r.Header.Get("Authorization")
	if auth != "" {
		// tolerate extra spaces
		parts := strings.Fields(auth)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	// Optional cookie fallback (comment out if you don’t want this behavior)
	if c, err := r.Cookie("access_token"); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

func RequireJWT(signer *secure.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			raw := bearerTokenFromRequest(r)
			if raw == "" {
				// RFC 6750 friendly header for auth failures
				w.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token", error_description="missing token"`)
				logger.Warn(ctx, "missing access token")
				helper.RespondError(w, r, apperror.Unauthorized("Missing access token"))
				return
			}

			claims, err := signer.ParseAccess(raw)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token", error_description="invalid or expired token"`)
				logger.Warn(ctx, "invalid or expired token", "error", err)
				helper.RespondError(w, r, apperror.Unauthorized("Invalid or expired token"))
				return
			}

			// Attach claims into context using secure helper
			ctx = secure.WithClaims(ctx, claims)
			logger.Debug(ctx, "JWT authenticated", "user_id", claims.UserID, "role", claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAnyRole allows access if the user's role matches ANY of the allowed roles.
// super_admin is always allowed by design (comment that branch if you don’t want it).
func RequireAnyRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allowedSet[strings.ToLower(strings.TrimSpace(a))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			claims, err := secure.ClaimsFromContext(ctx)
			if err != nil {
				logger.Error(ctx, "claims missing in context; RequireJWT must run first", "error", err)
				helper.RespondError(w, r, apperror.Unauthorized("Authentication required"))
				return
			}

			role := strings.ToLower(strings.TrimSpace(claims.Role))

			// super_admin bypass
			if role == "super_admin" {
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := allowedSet[role]; !ok {
				logger.Warn(ctx, "insufficient permissions", "user_id", claims.UserID, "role", role, "required_any", allowed)
				helper.RespondError(w, r, apperror.Forbidden("Insufficient permissions"))
				return
			}

			logger.Debug(ctx, "role authorization (any) passed", "user_id", claims.UserID, "role", role)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAllRoles is rarely needed, but provided for completeness; it requires
// the user’s role to match ALL provided roles (so usually used with 1 expected value).
func RequireAllRoles(required ...string) func(http.Handler) http.Handler {
	reqSet := make(map[string]struct{}, len(required))
	for _, a := range required {
		reqSet[strings.ToLower(strings.TrimSpace(a))] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			claims, err := secure.ClaimsFromContext(ctx)
			if err != nil {
				logger.Error(ctx, "claims missing in context; RequireJWT must run first", "error", err)
				helper.RespondError(w, r, apperror.Unauthorized("Authentication required"))
				return
			}

			role := strings.ToLower(strings.TrimSpace(claims.Role))

			// super_admin bypass
			if role == "super_admin" {
				next.ServeHTTP(w, r)
				return
			}

			// For a single required role, this equals exact match.
			// For multiple, this is only true if the same string appears multiple times,
			// which doesn’t usually make sense — prefer RequireAnyRole for multiple choices.
			for req := range reqSet {
				if role != req {
					logger.Warn(ctx, "insufficient permissions (all)", "user_id", claims.UserID, "role", role, "required_all", required)
					helper.RespondError(w, r, apperror.Forbidden("Insufficient permissions"))
					return
				}
			}

			logger.Debug(ctx, "role authorization (all) passed", "user_id", claims.UserID, "role", role)
			next.ServeHTTP(w, r)
		})
	}
}

// Convenience wrappers

func RequireRole(role ...string) func(http.Handler) http.Handler {
	return RequireAnyRole(role...)
}

func RequireAdminOrSuper() func(http.Handler) http.Handler {
	return RequireAnyRole("admin", "super_admin")
}

func RequireDriver() func(http.Handler) http.Handler {
	return RequireAnyRole("driver")
}

func RequireRider() func(http.Handler) http.Handler {
	return RequireAnyRole("rider")
}
