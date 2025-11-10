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
	UserStore              store.UserStore
	Signer                 *secure.Signer
	RefreshStore           store.RefreshStore
	AuthVerificationStore  store.AuthVerificationStore
	Mailer                 *mailer.Mailer
	DriverApplicationStore store.DriverApplicationStore
}

func NewUserHandler(
	us store.UserStore,
	signer *secure.Signer,
	rs store.RefreshStore,
	avs store.AuthVerificationStore,
	m *mailer.Mailer,
	das store.DriverApplicationStore,
) *UserHandler {
	return &UserHandler{
		UserStore:              us,
		Signer:                 signer,
		RefreshStore:           rs,
		AuthVerificationStore:  avs,
		Mailer:                 m,
		DriverApplicationStore: das,
	}
}

// ---------- LOGIN ----------

func (h *UserHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger.Debug(ctx, "login attempt started")

	ctxTO, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	email := helper.SanitizeEmail(body.Email)
	pw := strings.TrimSpace(body.Password)
	if !helper.IsValidEmail(email) {
		helper.RespondError(w, r, apperror.BadRequest("Invalid email address"))
		return
	}
	if !helper.IsValidPassword(pw) {
		helper.RespondError(w, r, apperror.BadRequest("Password must be at least 8 characters"))
		return
	}

	u, err := h.UserStore.GetByEmail(ctxTO, email)
	if err != nil {
		logger.Warn(ctx, "user lookup failed", "email", email, "error", err)
		logger.Audit(ctx, logger.AuditUserLogin, nil, helper.ClientIP(r), r.UserAgent(), false, map[string]any{
			"email":  email,
			"reason": "user_not_found",
		})
		helper.RespondError(w, r, apperror.InvalidCredentials())
		return
	}

	if !u.IsVerified {
		helper.RespondError(w, r, apperror.BadRequest("Email not verified"))
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

	accessToken, _, err := h.Signer.MintAccess(u.ID, helper.DerefOrString(u.Role, "rider"))
	if err != nil {
		logger.Error(ctx, "failed to mint access token", "user_id", u.ID, "error", err)
		helper.RespondError(w, r, apperror.InternalError("Failed to generate access token", err))
		return
	}

	ua := r.UserAgent()
	ip := helper.ClientIPNet(r)
	refreshTokenPlain, refreshRec, err := h.RefreshStore.Create(ctxTO, u.ID, ua, ip, 7*24*time.Hour, time.Now())
	if err != nil {
		logger.Error(ctx, "failed to create refresh token", "user_id", u.ID, "error", err)
		helper.RespondError(w, r, apperror.InternalError("Failed to create refresh token", err))
		return
	}

	logger.Info(ctx, "user logged in successfully", "user_id", u.ID, "refresh_token_id", refreshRec.ID)
	logger.Audit(ctx, logger.AuditUserLogin, &u.ID, helper.ClientIP(r), r.UserAgent(), true, map[string]any{"email": email})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshTokenPlain,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.EqualFold(os.Getenv("APP_ENV"), "production"),
		SameSite: http.SameSiteLaxMode, // consider Strict in prod if UX allows
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

// ---------- REGISTER (rider/driver) ----------

func (h *UserHandler) HandleRiderRegister(w http.ResponseWriter, r *http.Request) {
	h.handleRegister(w, r, "rider")
}

func (h *UserHandler) HandleDriverRegister(w http.ResponseWriter, r *http.Request) {
	h.handleRegister(w, r, "driver")
}

func (h *UserHandler) handleRegister(w http.ResponseWriter, r *http.Request, role string) {
	ctx := r.Context()
	logger.Info(ctx, "register attempt started")

	ctxTO, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	email := helper.SanitizeEmail(body.Email)
	pw := strings.TrimSpace(body.Password)
	if !helper.IsValidEmail(email) {
		helper.RespondError(w, r, apperror.BadRequest("Invalid email address"))
		return
	}
	if !helper.IsValidPassword(pw) {
		helper.RespondError(w, r, apperror.BadRequest("Password must be at least 8 characters"))
		return
	}

	exists, err := h.UserStore.VerifyEmailExists(ctxTO, email)
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
		// is_verified / is_active default to false in DB
	}
	u, err := h.UserStore.CreateUser(ctxTO, ureq)
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

	rawToken, _, err := h.AuthVerificationStore.Create(ctxTO, u.ID, purpose, ua, ip, 30*time.Minute, time.Now())
	if err != nil {
		logger.Error(ctx, "create verification token failed", "error", err)
		helper.RespondError(w, r, apperror.InternalError("Internal error", err))
		return
	}

	// Build verification URL (robust against missing scheme)
	base := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if base == "" {
		base = "http://localhost:8081"
	} else if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	verifyURL := base + "/api/v1/auth/verify?token=" + rawToken + "&purpose=" + string(purpose)

	// Send verification email (don't block user creation on SMTP failure)
	if err := mailer.SendVerificationEmail(ctx, h.Mailer, u.Email, "", role, verifyURL, "30 minutes"); err != nil {
		logger.Warn(ctx, "send verification email failed", "error", err)
	}

	helper.RespondMessage(w, r, http.StatusCreated, "Registration received. Check your email to confirm.")
}

// ---------- VERIFY EMAIL ----------

func (h *UserHandler) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctxTO, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	q := r.URL.Query()
	raw := strings.TrimSpace(q.Get("token"))
	purposeStr := strings.TrimSpace(q.Get("purpose"))

	// quick sanity checks (32 random bytes -> 64 hex chars typically)
	if raw == "" || len(raw) < 40 || len(raw) > 200 || purposeStr == "" {
		helper.RespondError(w, r, apperror.BadRequest("invalid token/purpose"))
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

	// Atomically validate & consume token
	tok, err := h.AuthVerificationStore.ValidateAndConsume(ctxTO, raw, purpose, time.Now())
	if err != nil {
		helper.RespondError(w, r, apperror.BadRequest("invalid or expired token"))
		return
	}

	// Mark verified
	if err := h.UserStore.SetVerified(ctxTO, tok.UserID, true); err != nil {
		helper.RespondError(w, r, apperror.InternalError("failed to mark verified", err))
		return
	}

	// Next steps per purpose
	switch purpose {
	case store.PurposeRiderConfirm:
		// auto-activate rider
		if err := h.UserStore.ActivateUser(ctxTO, tok.UserID); err != nil {
			helper.RespondError(w, r, apperror.InternalError("failed to activate rider", err))
			return
		}
		logger.Audit(ctxTO, "user_email_verified", &tok.UserID, helper.ClientIP(r), r.UserAgent(), true, map[string]any{"role": "rider"})
		helper.RespondMessage(w, r, http.StatusOK, "Email verified. Your rider account is now active.")

	case store.PurposeDriverConfirm:
		// create driver application (make Create idempotent on unique conflict)
		appRec, err := h.DriverApplicationStore.Create(ctxTO, tok.UserID, "driver email verified", time.Now().UTC())
		if err != nil {
			logger.Error(ctxTO, "create driver application failed", "error", err)
			helper.RespondError(w, r, apperror.InternalError("failed to create application", err))
			return
		}

		// notify admin (best-effort)
		adminLink := os.Getenv("ADMIN_CONSOLE_URL")
		if adminLink != "" {
			adminLink = strings.TrimRight(adminLink, "/") + "/driver-applications/" + appRec.ID.String()
		}

		if user, err := h.UserStore.GetByID(ctxTO, tok.UserID); err == nil {
			_ = mailer.SendDriverAdminAlert(
				ctxTO,
				h.Mailer,
				[]string{"info@luxsuv.us"},
				appRec.ID.String(), // <-- app id
				user.ID.String(),   // keep user id if you want both
				user.Email,
				time.Now().UTC().Format(time.RFC3339),
				os.Getenv("ADMIN_CONSOLE_URL"),
			)
		}
		logger.Audit(ctxTO, "user_email_verified", &tok.UserID, helper.ClientIP(r), r.UserAgent(), true, map[string]any{"role": "driver"})
		helper.RespondMessage(w, r, http.StatusOK, "Email verified. Your driver application is pending admin review.")
	}
}

// handle forgot and reset password
func (h *UserHandler) HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var body struct {
		Email string `json:"email"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		// Don't leak details; respond the same
		helper.RespondJSON(w, r, http.StatusAccepted, "If email exists, a reset link has been sent")
		return
	}

	email := helper.SanitizeEmail(body.Email)
	respondAccepted := func() {
		helper.RespondJSON(w, r, http.StatusAccepted, "If email exists, a reset link has been sent")
	}

	if !helper.IsValidEmail(email) {
		respondAccepted()
		return
	}

	// Look up user (do not leak existence)
	u, err := h.UserStore.GetByEmail(ctx, email)
	if err != nil {
		respondAccepted()
		return
	}

	// Restrict admins from using “forgot password” self-service
	role := strings.ToLower(helper.DerefOrString(u.Role, "rider"))
	if role == "admin" || role == "super_admin" {
		logger.Warn(ctx, "forgot-password attempted for admin class account", "email", email)
		_ = mailer.SendAlertAttemptAdminPasswordRecovery(ctx, h.Mailer, u.Email, helper.ClientIP(r), r.UserAgent())
		// TODO: add IP throttling / temporary ban here
		respondAccepted()
		return
	}

	// Issue token
	ua := r.UserAgent()
	ip := helper.ClientIPNet(r)
	raw, _, err := h.AuthVerificationStore.Create(ctx, u.ID, store.PurposePasswordReset, ua, ip, 15*time.Minute, time.Now().UTC())
	if err != nil {
		// Don’t leak failure
		respondAccepted()
		return
	}

	// Build reset URL
	base := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if base == "" {
		base = "http://localhost:8081"
	} else if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	resetURL := base + "/api/v1/auth/reset?token=" + raw

	// Send email (best-effort)
	_ = mailer.SendPasswordResetEmail(ctx, h.Mailer, u.Email, u.Email, resetURL)

	respondAccepted()
}
