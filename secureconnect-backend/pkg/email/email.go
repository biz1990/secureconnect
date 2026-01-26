package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/smtp"
	"time"

	"go.uber.org/zap"

	"secureconnect-backend/pkg/logger"
)

const (
	emailMIMEFormat = "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"BOUNDARY\"\r\n\r\n--BOUNDARY\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s\r\n--BOUNDARY\r\nContent-Type: text/html; charset=\"utf-8\"\r\n\r\n%s\r\n--BOUNDARY--\r\n"
)

// EmailType represents the type of email to send
type EmailType string

const (
	EmailTypeVerification  EmailType = "verification"
	EmailTypePasswordReset EmailType = "password_reset"
	EmailTypeWelcome       EmailType = "welcome"
	EmailTypeNotification  EmailType = "notification"
)

// Email represents an email to be sent
type Email struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// VerificationEmailData contains data for email verification
type VerificationEmailData struct {
	Username string
	Token    string
	NewEmail string
	AppURL   string
}

// PasswordResetEmailData contains data for password reset
type PasswordResetEmailData struct {
	Username string
	Token    string
	AppURL   string
}

// WelcomeEmailData contains data for welcome email
type WelcomeEmailData struct {
	Username string
	AppURL   string
}

// Sender defines the interface for sending emails
type Sender interface {
	Send(ctx context.Context, email *Email) error
	SendVerification(ctx context.Context, to string, data *VerificationEmailData) error
	SendPasswordReset(ctx context.Context, to string, data *PasswordResetEmailData) error
	SendWelcome(ctx context.Context, to string, data *WelcomeEmailData) error
}

// maskToken returns a safe masked version of a token for logging
// Shows only first 4 and last 4 characters, with middle masked
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// MockSender is a mock implementation for development/testing
type MockSender struct{}

// Send sends an email (mock implementation)
func (m *MockSender) Send(ctx context.Context, email *Email) error {
	logger.Info("Mock email sent",
		zap.String("to", email.To),
		zap.String("subject", email.Subject))
	return nil
}

// SendVerification sends a verification email (mock implementation)
func (m *MockSender) SendVerification(ctx context.Context, to string, data *VerificationEmailData) error {
	logger.Info("Mock verification email sent",
		zap.String("to", to),
		zap.String("username", data.Username),
		zap.String("token", maskToken(data.Token)))
	return nil
}

// SendPasswordReset sends a password reset email (mock implementation)
func (m *MockSender) SendPasswordReset(ctx context.Context, to string, data *PasswordResetEmailData) error {
	logger.Info("Mock password reset email sent",
		zap.String("to", to),
		zap.String("username", data.Username),
		zap.String("token", maskToken(data.Token)))
	return nil
}

// SendWelcome sends a welcome email (mock implementation)
func (m *MockSender) SendWelcome(ctx context.Context, to string, data *WelcomeEmailData) error {
	logger.Info("Mock welcome email sent",
		zap.String("to", to),
		zap.String("username", data.Username))
	return nil
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPSender sends emails via SMTP server
type SMTPSender struct {
	config *SMTPConfig
}

// NewSMTPSender creates a new SMTP sender
func NewSMTPSender(config *SMTPConfig) *SMTPSender {
	return &SMTPSender{
		config: config,
	}
}

// Send sends an email via SMTP
func (s *SMTPSender) Send(ctx context.Context, email *Email) error {
	// Create SMTP auth
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Build email message with both text and HTML parts
	message := fmt.Sprintf(emailMIMEFormat,
		s.config.From,
		email.To,
		email.Subject,
		email.Text,
		email.HTML,
	)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Use TLS for secure connection
	client, err := smtp.Dial(addr)
	if err != nil {
		logger.Error("Failed to connect to SMTP server",
			zap.String("host", s.config.Host),
			zap.Int("port", s.config.Port),
			zap.Error(err))
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Start TLS if available
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName:         s.config.Host,
			InsecureSkipVerify: false,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			logger.Error("Failed to start TLS",
				zap.String("host", s.config.Host),
				zap.Error(err))
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate
	if err := client.Auth(auth); err != nil {
		logger.Error("Failed to authenticate with SMTP server",
			zap.String("host", s.config.Host),
			zap.Error(err))
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Send email
	if err := client.Mail(s.config.From); err != nil {
		logger.Error("Failed to set sender",
			zap.String("from", s.config.From),
			zap.Error(err))
		return fmt.Errorf("failed to set sender: %w", err)
	}

	if err := client.Rcpt(email.To); err != nil {
		logger.Error("Failed to set recipient",
			zap.String("to", email.To),
			zap.Error(err))
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		logger.Error("Failed to get data writer",
			zap.Error(err))
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer wc.Close()

	_, err = io.WriteString(wc, message)
	if err != nil {
		logger.Error("Failed to write email message",
			zap.Error(err))
		return fmt.Errorf("failed to write email message: %w", err)
	}

	logger.Info("Email sent successfully",
		zap.String("to", email.To),
		zap.String("subject", email.Subject))
	return nil
}

// SendVerification sends a verification email via SMTP
func (s *SMTPSender) SendVerification(ctx context.Context, to string, data *VerificationEmailData) error {
	email := &Email{
		To:      to,
		Subject: "Verify Your Email Address - SecureConnect",
		HTML:    s.buildVerificationHTML(data),
		Text:    s.buildVerificationText(data),
	}
	return s.Send(ctx, email)
}

// SendPasswordReset sends a password reset email via SMTP
func (s *SMTPSender) SendPasswordReset(ctx context.Context, to string, data *PasswordResetEmailData) error {
	email := &Email{
		To:      to,
		Subject: "Reset Your Password - SecureConnect",
		HTML:    s.buildPasswordResetHTML(data),
		Text:    s.buildPasswordResetText(data),
	}
	return s.Send(ctx, email)
}

// SendWelcome sends a welcome email via SMTP
func (s *SMTPSender) SendWelcome(ctx context.Context, to string, data *WelcomeEmailData) error {
	email := &Email{
		To:      to,
		Subject: "Welcome to SecureConnect!",
		HTML:    s.buildWelcomeHTML(data),
		Text:    s.buildWelcomeText(data),
	}
	return s.Send(ctx, email)
}

// buildVerificationText builds plain text version of verification email
func (s *SMTPSender) buildVerificationText(data *VerificationEmailData) string {
	return fmt.Sprintf(`Hi %s,

You recently requested to change your email address on SecureConnect.

Please verify your new email address by clicking link below:

%s/verify-email?token=%s

If you didn't request this change, you can safely ignore this email.

This link will expire in 24 hours.

Best regards,
The SecureConnect Team`, data.Username, data.AppURL, data.Token)
}

// buildVerificationHTML builds HTML version of verification email
func (s *SMTPSender) buildVerificationHTML(data *VerificationEmailData) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verify Your Email - SecureConnect</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .container { background: #f9f9f9; padding: 40px 20px; border-radius: 8px; }
        .header { text-align: center; margin-bottom: 30px; }
        .logo { font-size: 24px; font-weight: bold; color: #4a90e2; }
        .content { background: #ffffff; padding: 30px; border-radius: 8px; }
        .button { display: inline-block; padding: 12px 30px; background: #4a90e2; color: #ffffff; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .button:hover { background: #3a7bc9; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">SecureConnect</div>
        </div>
        <div class="content">
            <h2>Verify Your Email Address</h2>
            <p>Hi %s,</p>
            <p>You recently requested to change your email address on SecureConnect.</p>
            <p>Please verify your new email address by clicking button below:</p>
            <p style="text-align: center;">
                <a href="%s/verify-email?token=%s" class="button">Verify Email</a>
            </p>
            <p><strong>This link will expire in 24 hours.</strong></p>
            <p>If you didn't request this change, you can safely ignore this email.</p>
        </div>
        <div class="footer">
            <p>&copy; %d SecureConnect. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, data.Username, data.AppURL, data.Token, time.Now().Year())
}

// buildPasswordResetText builds plain text version of password reset email
func (s *SMTPSender) buildPasswordResetText(data *PasswordResetEmailData) string {
	return fmt.Sprintf(`Hi %s,

You requested to reset your password on SecureConnect.

Click link below to reset your password:

%s/reset-password?token=%s

If you didn't request this change, you can safely ignore this email.

This link will expire in 1 hour.

Best regards,
The SecureConnect Team`, data.Username, data.AppURL, data.Token)
}

// buildPasswordResetHTML builds HTML version of password reset email
func (s *SMTPSender) buildPasswordResetHTML(data *PasswordResetEmailData) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Your Password - SecureConnect</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .container { background: #f9f9f9; padding: 40px 20px; border-radius: 8px; }
        .header { text-align: center; margin-bottom: 30px; }
        .logo { font-size: 24px; font-weight: bold; color: #4a90e2; }
        .content { background: #ffffff; padding: 30px; border-radius: 8px; }
        .button { display: inline-block; padding: 12px 30px; background: #4a90e2; color: #ffffff; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .button:hover { background: #3a7bc9; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">SecureConnect</div>
        </div>
        <div class="content">
            <h2>Reset Your Password</h2>
            <p>Hi %s,</p>
            <p>You requested to reset your password on SecureConnect.</p>
            <p>Click button below to reset your password:</p>
            <p style="text-align: center;">
                <a href="%s/reset-password?token=%s" class="button">Reset Password</a>
            </p>
            <p><strong>This link will expire in 1 hour.</strong></p>
            <p>If you didn't request this change, you can safely ignore this email.</p>
        </div>
        <div class="footer">
            <p>&copy; %d SecureConnect. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, data.Username, data.AppURL, data.Token, time.Now().Year())
}

// buildWelcomeText builds plain text version of welcome email
func (s *SMTPSender) buildWelcomeText(data *WelcomeEmailData) string {
	return fmt.Sprintf(`Hi %s,

Welcome to SecureConnect!

We're excited to have you on board. SecureConnect is a secure, end-to-end encrypted messaging platform that puts your privacy first.

Get started by visiting:

%s

If you have any questions, feel free to reach out to our support team.

Best regards,
The SecureConnect Team`, data.Username, data.AppURL)
}

// buildWelcomeHTML builds HTML version of welcome email
func (s *SMTPSender) buildWelcomeHTML(data *WelcomeEmailData) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to SecureConnect</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .container { background: #f9f9f9; padding: 40px 20px; border-radius: 8px; }
        .header { text-align: center; margin-bottom: 30px; }
        .logo { font-size: 24px; font-weight: bold; color: #4a90e2; }
        .content { background: #ffffff; padding: 30px; border-radius: 8px; }
        .button { display: inline-block; padding: 12px 30px; background: #4a90e2; color: #ffffff; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .button:hover { background: #3a7bc9; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">SecureConnect</div>
        </div>
        <div class="content">
            <h2>Welcome to SecureConnect!</h2>
            <p>Hi %s,</p>
            <p>We're excited to have you on board. SecureConnect is a secure, end-to-end encrypted messaging platform that puts your privacy first.</p>
            <p>Get started by visiting:</p>
            <p style="text-align: center;">
                <a href="%s" class="button">Get Started</a>
            </p>
            <p>If you have any questions, feel free to reach out to our support team.</p>
        </div>
        <div class="footer">
            <p>&copy; %d SecureConnect. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, data.Username, data.AppURL, time.Now().Year())
}

// Service handles email sending operations
type Service struct {
	sender Sender
}

// NewService creates a new email service
func NewService(sender Sender) *Service {
	return &Service{
		sender: sender,
	}
}

// SendVerificationEmail sends a verification email
func (s *Service) SendVerificationEmail(ctx context.Context, to string, data *VerificationEmailData) error {
	return s.sender.SendVerification(ctx, to, data)
}

// SendPasswordResetEmail sends a password reset email
func (s *Service) SendPasswordResetEmail(ctx context.Context, to string, data *PasswordResetEmailData) error {
	return s.sender.SendPasswordReset(ctx, to, data)
}

// SendWelcomeEmail sends a welcome email
func (s *Service) SendWelcomeEmail(ctx context.Context, to string, data *WelcomeEmailData) error {
	return s.sender.SendWelcome(ctx, to, data)
}
