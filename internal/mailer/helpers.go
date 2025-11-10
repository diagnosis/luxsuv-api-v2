package mailer

import (
	"context"
	"os"
	"time"
)

func SendWelcomeEmail(ctx context.Context, m *Mailer, toEmail, userName string) error {
	templateData := map[string]string{
		"Name":  userName,
		"Email": toEmail,
	}

	return m.SendTemplate(
		ctx,
		[]string{toEmail},
		"Welcome to Lux SUV",
		WelcomeEmailTemplate,
		templateData,
	)
}

func SendPasswordResetEmail(ctx context.Context, m *Mailer, toEmail, userName, resetLink string) error {
	templateData := map[string]string{
		"Name":      userName,
		"ResetLink": resetLink,
	}

	return m.SendTemplate(
		ctx,
		[]string{toEmail},
		"Password Reset Request",
		PasswordResetTemplate,
		templateData,
	)
}

func SendLoginAlertEmail(ctx context.Context, m *Mailer, toEmail, userName, loginTime, ipAddress, userAgent string) error {
	templateData := map[string]string{
		"Name":      userName,
		"Time":      loginTime,
		"IPAddress": ipAddress,
		"UserAgent": userAgent,
	}

	return m.SendTemplate(
		ctx,
		[]string{toEmail},
		"New Login Detected",
		LoginAlertTemplate,
		templateData,
	)
}
func SendVerificationEmail(ctx context.Context, m *Mailer, toEmail, name, role, verifyLink, expiresIn string) error {
	return m.SendTemplate(ctx, []string{toEmail}, "Verify your "+role+" account",
		VerifyAccountTemplate, map[string]any{
			"Name":       name,
			"Role":       role,
			"VerifyLink": verifyLink,
			"ExpiresIn":  expiresIn, // e.g., "30 minutes"
		})
}

// mailer
func SendDriverAdminAlert(ctx context.Context, m *Mailer, adminEmails []string,
	appID, userID, email, verifiedAt, adminLink string) error {

	data := map[string]any{
		"AppID":      appID,
		"UserID":     userID,
		"Email":      email,
		"VerifiedAt": verifiedAt,
		"AdminLink":  adminLink,
	}
	return m.SendTemplate(ctx, adminEmails,
		"Driver email verified – review application",
		DriverVerifiedAdminAlertTemplate, data)
}

func SendAlertAttemptAdminPasswordRecovery(ctx context.Context, m *Mailer, adminEmail string, ipAddress string, userAgent string) error {
	if m == nil || !m.IsConfigured() || len(adminEmail) == 0 {
		return nil
	}
	data := struct {
		AttemptTime     string
		AdminEmail      string
		IPAddress       string
		UserAgent       string
		AdminConsoleURL string
	}{
		AttemptTime:     time.Now().Format(time.RFC3339),
		AdminEmail:      adminEmail,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
		AdminConsoleURL: os.Getenv("ADMIN_CONSOLE_URL"),
	}

	return m.SendTemplate(ctx, []string{adminEmail}, "Alert: Admin password recovery attempt", SuspiciousAdminRecoveryAlertTemplate, data)
}
