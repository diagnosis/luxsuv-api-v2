package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/apperror"
	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/mailer"
	"github.com/diagnosis/luxsuv-api-v2/internal/secure"
	"github.com/diagnosis/luxsuv-api-v2/internal/store"
)

type UserHandler struct {
	UserStore             store.UserStore
	Signer                *secure.Signer
	RefreshStore          store.RefreshStore
	AuthVerificationStore store.AuthVerificationStore
	Mailer                *mailer.Mailer
}

func NewUserHandler(us store.UserStore, signer *secure.Signer, rs store.RefreshStore, avs store.AuthVerificationStore, mailer *mailer.Mailer) *UserHandler {
	return &UserHandler{us, signer, rs, avs, mailer}
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
		logger.Error(ctx, "failed to parse login request", "error", err)
		helper.RespondError(w, r, apperror.BadRequest("Invalid request body"))
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
	if !u.IsVerified {
		helper.RespondError(w, r, apperror.BadRequest("Email not verified"))
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

	accessToken, _, err := h.Signer.MintAccess(u.ID, helper.DerefOrString(u.Role, "rider"))
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
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshTokenPlain,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.EqualFold(os.Getenv("APP_ENV"), "production"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	response := map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(h.Signer.AccessTTL.Seconds()),
		"user": map[string]any{
			"id":    u.ID,
			"email": u.Email,
			"role":  helper.DerefOrString(u.Role, "rider"),
		},
	}

	helper.RespondJSON(w, r, http.StatusOK, response)
}
func (h *UserHandler) HandleRiderRegister(w http.ResponseWriter, r *http.Request) {
	h.handleRegister(w, r, "rider")
}
func (h *UserHandler) HandleDriverRegister(w http.ResponseWriter, r *http.Request) {
	h.handleRegister(w, r, "driver")
}

// handle driver registration
func (h *UserHandler) handleRegister(w http.ResponseWriter, r *http.Request, role string) {
	ctx := r.Context()
	logger.Info(ctx, "register attempt started")

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
		logger.Error(ctx, "failed to parse registration request", "error", err)
		helper.RespondError(w, r, apperror.BadRequest("Invalid request body"))
		return
	}
	defer r.Body.Close()

	email := strings.ToLower(strings.TrimSpace(body.Email))
	pw := strings.TrimSpace(body.Password)
	if len(email) < 4 || len(pw) < 8 {
		helper.RespondError(w, r, apperror.BadRequest("Email must be ≥4 chars and password ≥8 chars"))
		return
	}

	exists, err := h.UserStore.VerifyEmailExists(ctxTimeout, email)
	if err != nil {
		helper.RespondError(w, r, apperror.InternalError("Internal error", err))
		return
	}
	if exists {
		helper.RespondError(w, r, apperror.EmailAlreadyExists())
		return
	}

	pwHash, err := secure.HashPassword(pw)
	if err != nil {
		helper.RespondError(w, r, apperror.InternalError("Internal error", err))
		return
	}

	ureq := &store.User{
		Email:        email,
		PasswordHash: pwHash,
		Role:         &role, // "rider" or "driver"
		// IsActive/IsVerified default to false in DB
	}
	u, err := h.UserStore.CreateUser(ctxTimeout, ureq)
	if err != nil {
		logger.Error(ctx, "create user failed", "error", err)
		helper.RespondError(w, r, apperror.InternalError("Internal error", err))
		return
	}

	// Issue verification token
	ua := r.UserAgent()
	ip := helper.ClientIPNet(r)

	var purpose store.AVPurpose
	switch role {
	case "rider":
		purpose = store.PurposeRiderConfirm
	case "driver":
		purpose = store.PurposeDriverConfirm
	default:
		helper.RespondError(w, r, apperror.BadRequest("invalid role"))
		return
	}

	rawToken, _, err := h.AuthVerificationStore.Create(ctxTimeout, u.ID, purpose, ua, ip, 30*time.Minute, time.Now())
	if err != nil {
		logger.Error(ctx, "create verification token failed", "error", err)
		helper.RespondError(w, r, apperror.InternalError("Internal error", err))
		return
	}

	// Build verification link to your API endpoint
	base := strings.TrimRight(os.Getenv("APP_DOMAIN"), "/")
	if base == "" {
		base = "http://localhost:8081" // your API port in dev
	}
	verifyURL := base + "/api/v1/auth/verify?token=" + rawToken + "&purpose=" + string(purpose)

	// Send email (keep welcome optional)
	_ = h.Mailer.SendTemplate(ctx, []string{u.Email}, "Confirm your account",
		`<p>Welcome to LuxSUV!</p>
         <p>Please confirm your email by clicking the link below:</p>
         <p><a href="{{.Link}}">{{.Link}}</a></p>`,
		map[string]any{"Link": verifyURL},
	)

	helper.RespondMessage(w, r, http.StatusCreated, "Registration received. Check your email to confirm.")
}

// verify email handler
func (h *UserHandler) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	raw := strings.TrimSpace(q.Get("token"))
	purposeStr := strings.TrimSpace(q.Get("purpose"))

	if raw == "" || purposeStr == "" {
		helper.RespondError(w, r, apperror.BadRequest("missing token or purpose"))
		return
	}

	var purpose store.AVPurpose
	switch purposeStr {
	case string(store.PurposeRiderConfirm):
		purpose = store.PurposeRiderConfirm
	case string(store.PurposeDriverConfirm):
		purpose = store.PurposeDriverConfirm
	default:
		helper.RespondError(w, r, apperror.BadRequest("invalid purpose"))
		return
	}

	// Consume token
	tok, err := h.AuthVerificationStore.ValidateAndConsume(ctx, raw, purpose, time.Now())
	if err != nil {
		helper.RespondError(w, r, apperror.BadRequest("invalid or expired token"))
		return
	}

	// Mark user verified
	if err := h.UserStore.SetVerified(ctx, tok.UserID, true); err != nil {
		helper.RespondError(w, r, apperror.InternalError("failed to mark verified", err))
		return
	}

	// Rider: auto-activate; Driver: leave inactive & create/persist driver application (next step)
	switch purpose {
	case store.PurposeRiderConfirm:
		if err := h.UserStore.ActivateUser(ctx, tok.UserID); err != nil {
			helper.RespondError(w, r, apperror.InternalError("failed to activate rider", err))
			return
		}
		helper.RespondMessage(w, r, http.StatusOK, "Email verified. Your rider account is now active.")
	case store.PurposeDriverConfirm:
		// TODO: ensure driver_applications row exists (pending), and notify admins via email.
		helper.RespondMessage(w, r, http.StatusOK, "Email verified. Your driver application is pending admin review.")
	}
}
