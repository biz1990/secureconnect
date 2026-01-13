package email

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"secureconnect-backend/pkg/logger"
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
		zap.String("token", data.Token))
	return nil
}

// SendPasswordReset sends a password reset email (mock implementation)
func (m *MockSender) SendPasswordReset(ctx context.Context, to string, data *PasswordResetEmailData) error {
	logger.Info("Mock password reset email sent",
		zap.String("to", to),
		zap.String("username", data.Username),
		zap.String("token", data.Token))
	return nil
}

// SendWelcome sends a welcome email (mock implementation)
func (m *MockSender) SendWelcome(ctx context.Context, to string, data *WelcomeEmailData) error {
	logger.Info("Mock welcome email sent",
		zap.String("to", to),
		zap.String("username", data.Username))
	return nil
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

// buildVerificationText builds the plain text version of verification email
func (s *Service) buildVerificationText(data *VerificationEmailData) string {
	return fmt.Sprintf(`Hi %s,

You recently requested to change your email address on SecureConnect.

Please verify your new email address by clicking the link below:

%s/verify-email?token=%s

If you didn't request this change, you can safely ignore this email.

This link will expire in 24 hours.

Best regards,
The SecureConnect Team`, data.Username, data.AppURL, data.Token)
}

// buildVerificationHTML builds the HTML version of verification email
func (s *Service) buildVerificationHTML(data *VerificationEmailData) string {
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
            <p>Please verify your new email address by clicking the button below:</p>
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

// buildPasswordResetText builds the plain text version of password reset email
func (s *Service) buildPasswordResetText(data *PasswordResetEmailData) string {
	return fmt.Sprintf(`Hi %s,

You requested to reset your password on SecureConnect.

Click the link below to reset your password:

%s/reset-password?token=%s

If you didn't request this change, you can safely ignore this email.

This link will expire in 1 hour.

Best regards,
The SecureConnect Team`, data.Username, data.AppURL, data.Token)
}

// buildPasswordResetHTML builds the HTML version of password reset email
func (s *Service) buildPasswordResetHTML(data *PasswordResetEmailData) string {
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
            <p>Click the button below to reset your password:</p>
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

// buildWelcomeText builds the plain text version of welcome email
func (s *Service) buildWelcomeText(data *WelcomeEmailData) string {
	return fmt.Sprintf(`Hi %s,

Welcome to SecureConnect!

We're excited to have you on board. SecureConnect is a secure, end-to-end encrypted messaging platform that puts your privacy first.

Get started by visiting:

%s

If you have any questions, feel free to reach out to our support team.

Best regards,
The SecureConnect Team`, data.Username, data.AppURL)
}

// buildWelcomeHTML builds the HTML version of welcome email
func (s *Service) buildWelcomeHTML(data *WelcomeEmailData) string {
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
