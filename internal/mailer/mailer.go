package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strings"

	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
)

type Mailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func NewMailer() *Mailer {
	return &Mailer{
		host:     os.Getenv("SMTP_HOST"),
		port:     os.Getenv("SMTP_PORT"),
		username: os.Getenv("SMTP_USERNAME"),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     os.Getenv("SMTP_FROM"),
	}
}

func (m *Mailer) IsConfigured() bool {
	return m.host != "" && m.port != "" && m.username != "" && m.password != "" && m.from != ""
}

type EmailData struct {
	To      []string
	Subject string
	Body    string
	IsHTML  bool
}

func (m *Mailer) Send(ctx context.Context, data EmailData) error {
	if !m.IsConfigured() {
		logger.Warn(ctx, "SMTP not configured, skipping email send")
		return fmt.Errorf("SMTP not configured")
	}

	if len(data.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	msg := m.buildMessage(data)

	auth := smtp.PlainAuth("", m.username, m.password, m.host)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	tlsConfig := &tls.Config{
		ServerName: m.host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		logger.Error(ctx, "failed to establish TLS connection", "error", err)
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		logger.Error(ctx, "failed to create SMTP client", "error", err)
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	if err = client.Auth(auth); err != nil {
		logger.Error(ctx, "SMTP authentication failed", "error", err)
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	if err = client.Mail(m.from); err != nil {
		logger.Error(ctx, "failed to set sender", "error", err)
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, recipient := range data.To {
		if err = client.Rcpt(recipient); err != nil {
			logger.Error(ctx, "failed to add recipient", "recipient", recipient, "error", err)
			return fmt.Errorf("failed to add recipient %s: %w", recipient, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		logger.Error(ctx, "failed to initialize data transfer", "error", err)
		return fmt.Errorf("failed to initialize data transfer: %w", err)
	}

	_, err = writer.Write([]byte(msg))
	if err != nil {
		logger.Error(ctx, "failed to write message", "error", err)
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = writer.Close()
	if err != nil {
		logger.Error(ctx, "failed to close data writer", "error", err)
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	logger.Info(ctx, "email sent successfully", "to", strings.Join(data.To, ", "), "subject", data.Subject)
	return nil
}

func (m *Mailer) buildMessage(data EmailData) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("From: %s\r\n", m.from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(data.To, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", data.Subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	if data.IsHTML {
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}

	buf.WriteString("\r\n")
	buf.WriteString(data.Body)

	return buf.String()
}

func (m *Mailer) SendTemplate(ctx context.Context, to []string, subject string, templateStr string, templateData any) error {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		logger.Error(ctx, "failed to parse email template", "error", err)
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		logger.Error(ctx, "failed to execute email template", "error", err)
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return m.Send(ctx, EmailData{
		To:      to,
		Subject: subject,
		Body:    buf.String(),
		IsHTML:  true,
	})
}
