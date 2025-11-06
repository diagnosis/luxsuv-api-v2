package logger

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
)

type AuditEvent string

const (
	AuditUserLogin         AuditEvent = "USER_LOGIN"
	AuditUserLogout        AuditEvent = "USER_LOGOUT"
	AuditUserRegistration  AuditEvent = "USER_REGISTRATION"
	AuditPasswordChange    AuditEvent = "PASSWORD_CHANGE"
	AuditPasswordReset     AuditEvent = "PASSWORD_RESET"
	AuditTokenRefresh      AuditEvent = "TOKEN_REFRESH"
	AuditTokenRevoke       AuditEvent = "TOKEN_REVOKE"
	AuditAccountActivate   AuditEvent = "ACCOUNT_ACTIVATE"
	AuditAccountDeactivate AuditEvent = "ACCOUNT_DEACTIVATE"
)

var auditLogger *slog.Logger

func init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	auditLogger = slog.New(handler)
}

func Audit(ctx context.Context, event AuditEvent, userID *uuid.UUID, ip, userAgent string, success bool, details map[string]any) {
	correlationID := GetCorrelationID(ctx)
	attrs := []any{
		"type", "audit",
		"timestamp", time.Now().UTC(),
		"event", event,
		"correlation_id", correlationID,
		"ip", redactIP(ip),
		"user_agent", redactUserAgent(userAgent),
		"success", success,
	}
	if userID != nil {
		attrs = append(attrs, "user_id", userID.String())
	}
	if details != nil {
		for k, v := range details {
			attrs = append(attrs, k, v)
		}
	}
	auditLogger.InfoContext(ctx, "audit_event", attrs...)
}
