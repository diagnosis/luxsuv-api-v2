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

type MailerType string

const (
	MailerTypeLocal MailerType = "local"
	MailerTypeZepto MailerType = "zepto"
)

type Mailer struct {
	mailerType MailerType
	host       string
	port       string
	username   string
	password   string
	from       string
	useTLS     bool // true means "secure channel": STARTTLS on 587 or implicit TLS on 465
}

func NewMailer() *Mailer {
	logger.Info(context.Background(), "mailer starting",
		"host", os.Getenv("SMTP_HOST"),
		"port", os.Getenv("SMTP_PORT"),
		"useTLS", os.Getenv("SMTP_USE_TLS"),
	)

	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	useTLS := strings.EqualFold(strings.TrimSpace(os.Getenv("SMTP_USE_TLS")), "true")

	// Defaults
	if port == "" {
		if useTLS {
			port = "587" // Zepto default (STARTTLS)
		} else {
			port = "1025" // Mailpit default (plain)
		}
	}
	if host == "" {
		if useTLS {
			host = "smtp.zeptomail.com"
		} else {
			host = "127.0.0.1"
		}
	}

	mt := MailerTypeLocal
	if useTLS {
		mt = MailerTypeZepto
	}

	return &Mailer{
		mailerType: mt,
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		from:       from,
		useTLS:     useTLS,
	}
}

// Dev-friendly: in plain mode we don't require username/password.
func (m *Mailer) IsConfigured() bool {
	if m.host == "" || m.port == "" || m.from == "" {
		return false
	}
	if m.useTLS {
		return m.username != "" && m.password != ""
	}
	return true
}

func (m *Mailer) GetMailerType() MailerType { return m.mailerType }

type EmailData struct {
	To      []string
	Subject string
	Body    string
	IsHTML  bool
}

func (m *Mailer) Send(ctx context.Context, data EmailData) error {
	if !m.IsConfigured() {
		return fmt.Errorf("SMTP not configured")
	}
	if len(data.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	// Route selection: prefer semantics (useTLS) + port.
	logger.Info(ctx, "mailer send path", "host", m.host, "port", m.port, "useTLS", m.useTLS)

	switch {
	// Zepto standard: STARTTLS on 587
	case m.useTLS && m.port == "587":
		return m.sendWithSTARTTLS(ctx, data)

	// Implicit TLS (465)
	case m.useTLS && m.port == "465":
		return m.sendWithTLS(ctx, data)

	// Plain (Mailpit 1025 or any non-TLS local)
	default:
		return m.sendPlain(ctx, data)
	}
}

// Plain, no TLS (Mailpit: 1025). AUTH optional if creds present.
func (m *Mailer) sendPlain(ctx context.Context, data EmailData) error {
	msg := m.buildMessage(data)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	c, err := smtp.Dial(addr)
	if err != nil {
		logger.Error(ctx, "smtp dial (plain) failed", "error", err)
		return fmt.Errorf("dial SMTP: %w", err)
	}
	defer c.Quit()

	// AUTH only if creds provided (Mailpit typically doesn't use auth)
	if m.username != "" && m.password != "" {
		if err := c.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			logger.Error(ctx, "smtp auth (plain) failed", "error", err)
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	for _, rcpt := range data.To {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to initialize data transfer: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	logger.Info(ctx, "email sent (plain)", "to", strings.Join(data.To, ", "), "subject", data.Subject)
	return nil
}

// STARTTLS on 587 (upgrade after EHLO), with AUTH if provided.
func (m *Mailer) sendWithSTARTTLS(ctx context.Context, data EmailData) error {
	msg := m.buildMessage(data)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	c, err := smtp.Dial(addr)
	if err != nil {
		logger.Error(ctx, "smtp dial (starttls) failed", "error", err)
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer c.Quit()

	// EHLO/HELO to populate server extensions
	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}
	if ok, _ := c.Extension("STARTTLS"); !ok {
		return fmt.Errorf("server does not advertise STARTTLS")
	}
	if err := c.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
		logger.Error(ctx, "failed to start TLS (starttls)", "error", err)
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// After STARTTLS, authenticate if creds provided
	if m.username != "" && m.password != "" {
		// Extension("AUTH") often true post-STARTTLS; if false, still try AUTH.
		if err := c.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			logger.Error(ctx, "smtp auth (starttls) failed", "error", err)
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	for _, rcpt := range data.To {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", rcpt, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to initialize data transfer: %w", err)
	}
	if _, err = w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	logger.Info(ctx, "email sent (STARTTLS)", "host", m.host, "port", m.port, "to", strings.Join(data.To, ", "), "subject", data.Subject)
	return nil
}

// Implicit TLS on 465, with AUTH.
func (m *Mailer) sendWithTLS(ctx context.Context, data EmailData) error {
	msg := m.buildMessage(data)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.host})
	if err != nil {
		logger.Error(ctx, "tls dial (implicit) failed", "error", err)
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		logger.Error(ctx, "smtp client (implicit) failed", "error", err)
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Quit()

	if m.username != "" && m.password != "" {
		if err := c.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			logger.Error(ctx, "smtp auth (implicit) failed", "error", err)
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	for _, rcpt := range data.To {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", rcpt, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to initialize data transfer: %w", err)
	}
	if _, err = w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	logger.Info(ctx, "email sent (implicit TLS)", "host", m.host, "port", m.port, "to", strings.Join(data.To, ", "), "subject", data.Subject)
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
