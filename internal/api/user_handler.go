package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/apperror"
	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/secure"
	"github.com/diagnosis/luxsuv-api-v2/internal/store"
)

type UserHandler struct {
	UserStore    store.UserStore
	Signer       *secure.Signer
	RefreshStore store.RefreshStore
}

func NewUserHandler(us store.UserStore, signer *secure.Signer, rs store.RefreshStore) *UserHandler {
	return &UserHandler{us, signer, rs}
}

func (h *UserHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger.Debug(ctx, "login attempt started")

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		helper.RespondError(w, r, apperror.BadRequest("Invalid request body"))
		logger.Error(ctx, "failed to parse login request", "error", err)
		return
	}
	defer r.Body.Close()

	email := strings.ToLower(strings.TrimSpace(body.Email))
	pw := strings.TrimSpace(body.Password)

	if len(email) < 4 || len(pw) < 8 {
		logger.Warn(ctx, "login validation failed", "email_len", len(email), "pw_len", len(pw))
		helper.RespondError(w, r, apperror.BadRequest("Email must be at least 4 characters and password at least 8 characters"))
		return
	}

	u, err := h.UserStore.GetByEmail(ctxTimeout, email)
	if err != nil {
		logger.Warn(ctx, "user lookup failed", "email", email, "error", err)
		logger.Audit(ctx, logger.AuditUserLogin, nil, helper.ClientIP(r), r.UserAgent(), false, map[string]any{
			"email":  email,
			"reason": "user_not_found",
		})
		helper.RespondError(w, r, apperror.InvalidCredentials())
		return
	}

	if !u.IsActive {
		logger.Warn(ctx, "inactive account login attempt", "user_id", u.ID)
		logger.Audit(ctx, logger.AuditUserLogin, &u.ID, helper.ClientIP(r), r.UserAgent(), false, map[string]any{
			"email":  email,
			"reason": "account_inactive",
		})
		helper.RespondError(w, r, apperror.AccountInactive())
		return
	}

	if !secure.VerifyPassword(pw, u.PasswordHash) {
		logger.Warn(ctx, "invalid password", "user_id", u.ID)
		logger.Audit(ctx, logger.AuditUserLogin, &u.ID, helper.ClientIP(r), r.UserAgent(), false, map[string]any{
			"email":  email,
			"reason": "invalid_password",
		})
		helper.RespondError(w, r, apperror.InvalidCredentials())
		return
	}

	accessToken, _, err := h.Signer.MintAccess(u.ID, helper.DeferOrString(u.Role, "rider"))
	if err != nil {
		helper.RespondError(w, r, apperror.InternalError("Failed to generate access token", err))
		logger.Error(ctx, "failed to mint access token", "user_id", u.ID, "error", err)
		return
	}

	ua := r.UserAgent()
	ip := helper.ClientIPNet(r)
	refreshTokenPlain, refreshRec, err := h.RefreshStore.Create(ctxTimeout, u.ID, ua, ip, 7*24*time.Hour, time.Now())
	if err != nil {
		helper.RespondError(w, r, apperror.InternalError("Failed to create refresh token", err))
		logger.Error(ctx, "failed to create refresh token", "user_id", u.ID, "error", err)
		return
	}

	logger.Info(ctx, "user logged in successfully", "user_id", u.ID, "refresh_token_id", refreshRec.ID)
	logger.Audit(ctx, logger.AuditUserLogin, &u.ID, helper.ClientIP(r), r.UserAgent(), true, map[string]any{
		"email": email,
	})

	response := map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshTokenPlain,
		"token_type":    "Bearer",
		"expires_in":    int(h.Signer.AccessTTL.Seconds()),
		"user": map[string]any{
			"id":    u.ID,
			"email": u.Email,
			"role":  helper.DeferOrString(u.Role, "rider"),
		},
	}

	helper.RespondJSON(w, r, http.StatusOK, response)
}
