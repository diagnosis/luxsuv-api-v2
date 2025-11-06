package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/diagnosis/luxsuv-api-v2/internal/apperror"
	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/secure"
	"github.com/google/uuid"
)

type ctxKey string

const (
	userIDKey ctxKey = "user_id"
	userRole  ctxKey = "user_role"
)

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func GetUserRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(userRole).(string)
	return role, ok
}

func RequireJWT(signer *secure.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn(ctx, "missing authorization header")
				helper.RespondError(w, r, apperror.Unauthorized("Missing authorization header"))
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				logger.Warn(ctx, "invalid authorization header format")
				helper.RespondError(w, r, apperror.Unauthorized("Invalid authorization header format"))
				return
			}

			token := parts[1]
			claims, err := signer.ParseAccess(token)
			if err != nil {
				logger.Warn(ctx, "invalid or expired token", "error", err)
				helper.RespondError(w, r, apperror.Unauthorized("Invalid or expired token"))
				return
			}

			ctx = context.WithValue(ctx, userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userRole, claims.Role)

			logger.Debug(ctx, "JWT authenticated", "user_id", claims.UserID, "role", claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			userID, ok := GetUserID(ctx)
			if !ok {
				logger.Error(ctx, "user_id not found in context - RequireJWT must be applied first")
				helper.RespondError(w, r, apperror.Unauthorized("Authentication required"))
				return
			}

			role, ok := GetUserRole(ctx)
			if !ok {
				logger.Error(ctx, "user_role not found in context", "user_id", userID)
				helper.RespondError(w, r, apperror.Unauthorized("Authentication required"))
				return
			}

			allowed := false
			for _, allowedRole := range allowedRoles {
				if role == allowedRole {
					allowed = true
					break
				}
			}

			if !allowed {
				logger.Warn(ctx, "insufficient permissions", "user_id", userID, "role", role, "required_roles", allowedRoles)
				helper.RespondError(w, r, apperror.Forbidden("Insufficient permissions"))
				return
			}

			logger.Debug(ctx, "role authorization passed", "user_id", userID, "role", role)

			next.ServeHTTP(w, r)
		})
	}
}
