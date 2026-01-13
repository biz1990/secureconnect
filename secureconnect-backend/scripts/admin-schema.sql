-- =============================================================================
-- ADMINISTRATION SCHEMA
-- CockroachDB Schema for SecureConnect Administration
-- =============================================================================

-- User Bans Table
CREATE TABLE IF NOT EXISTS user_bans (
    ban_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    banned_by UUID NOT NULL,
    ban_reason VARCHAR(500) NOT NULL,
    banned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    banned_until TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    unbanned_at TIMESTAMPTZ,
    unbanned_by UUID,

    CONSTRAINT fk_user_bans_user FOREIGN KEY (user_id)
        REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT fk_user_bans_banned_by FOREIGN KEY (banned_by)
        REFERENCES users(user_id) ON DELETE SET NULL,
    CONSTRAINT fk_user_bans_unbanned_by FOREIGN KEY (unbanned_by)
        REFERENCES users(user_id) ON DELETE SET NULL
);

-- Indexes for efficient queries
CREATE INDEX idx_user_bans_user ON user_bans(user_id, is_active);
CREATE INDEX idx_user_bans_active ON user_bans(is_active, banned_at);

-- Audit Logs Table
CREATE TABLE IF NOT EXISTS audit_logs (
    audit_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID NOT NULL,
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    details TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_audit_logs_admin FOREIGN KEY (admin_id)
        REFERENCES users(user_id) ON DELETE SET NULL
);

-- Indexes for efficient queries
CREATE INDEX idx_audit_logs_admin ON audit_logs(admin_id, created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs(action, created_at DESC);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id, created_at DESC);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at DESC);

-- Conversation Stats Table (for message statistics)
CREATE TABLE IF NOT EXISTS conversation_stats (
    conversation_id UUID PRIMARY KEY,
    message_count BIGINT NOT NULL DEFAULT 0,
    last_message_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_conversation_stats_conv FOREIGN KEY (conversation_id)
        REFERENCES conversations(conversation_id) ON DELETE CASCADE
);

-- Add role column to users table if not exists
ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin'));

-- Add last_login_at column to users table if not exists
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

-- Add storage_quota_used column to users table if not exists
ALTER TABLE users ADD COLUMN IF NOT EXISTS storage_quota_used BIGINT NOT NULL DEFAULT 0;

-- Create index on users table for admin queries
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

-- Grant admin role to existing admin users (run this once after migration)
-- UPDATE users SET role = 'admin' WHERE email IN ('admin@secureconnect.com');
