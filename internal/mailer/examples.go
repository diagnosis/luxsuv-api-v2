package mailer

import "context"

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
