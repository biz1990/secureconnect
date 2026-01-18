package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// AuthMetrics contains all authentication-related metrics
var (
	// Login metrics
	AuthLoginSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_login_success_total",
		Help: "Total number of successful login attempts",
	})

	AuthLoginFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_login_failed_total",
		Help: "Total number of failed login attempts",
	})

	AuthLoginFailedByIP = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_login_failed_by_ip_total",
		Help: "Total number of failed login attempts by IP address",
	}, []string{"ip"})

	AuthLoginDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "auth_login_duration_seconds",
		Help:    "Histogram of login request duration in seconds",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})

	// Account lockout metrics
	AuthAccountLockedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_account_locked_total",
		Help: "Total number of accounts locked due to failed login attempts",
	})

	AuthBruteForceDetectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_brute_force_detected_total",
		Help: "Total number of brute-force attacks detected",
	})

	// Refresh token metrics
	AuthRefreshTokenSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_refresh_token_success_total",
		Help: "Total number of successful token refreshes",
	})

	AuthRefreshTokenInvalidTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_refresh_token_invalid_total",
		Help: "Total number of invalid refresh token attempts",
	})

	AuthRefreshTokenBlacklistedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_refresh_token_blacklisted_total",
		Help: "Total number of refresh tokens blacklisted",
	})

	// Logout metrics
	AuthLogoutTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_logout_total",
		Help: "Total number of logout operations",
	})

	// Token revocation metrics
	AuthTokenBlacklistedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_token_blacklisted_total",
		Help: "Total number of tokens blacklisted",
	})

	// Rate limiting metrics
	AuthRateLimitExceededTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_rate_limit_exceeded_total",
		Help: "Total number of rate limit violations",
	}, []string{"endpoint"})

	// Password reset metrics
	AuthPasswordResetTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_password_reset_total",
		Help: "Total number of password reset requests",
	})

	AuthPasswordResetRateLimitExceededTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_password_reset_rate_limit_exceeded_total",
		Help: "Total number of password reset rate limit violations",
	})

	// Session metrics
	AuthSessionLimitExceededTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_session_limit_exceeded_total",
		Help: "Total number of session limit violations",
	})

	AuthActiveSessionsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "auth_active_sessions_total",
		Help: "Current number of active sessions",
	})
)
