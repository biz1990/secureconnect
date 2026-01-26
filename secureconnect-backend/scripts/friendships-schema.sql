-- SecureConnect Friendships and Blocked Users Schema
-- This file contains the SQL schema for user relationships
-- Version: 1.0

-- ==========================================
-- 1. FRIENDSHIPS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS friendships (
    user_id_1 UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    user_id_2 UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status STRING NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id_1, user_id_2),
    CONSTRAINT friendships_status_check CHECK (status IN ('pending', 'accepted', 'rejected', 'blocked')),
    CONSTRAINT friendships_different_users CHECK (user_id_1 != user_id_2),
    INDEX idx_friendships_user_id_1 (user_id_1),
    INDEX idx_friendships_user_id_2 (user_id_2),
    INDEX idx_friendships_status (status)
);

-- ==========================================
-- 2. BLOCKED USERS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS blocked_users (
    blocker_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    reason STRING,
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CONSTRAINT blocked_users_different_users CHECK (blocker_id != blocked_id),
    INDEX idx_blocked_users_blocker_id (blocker_id),
    INDEX idx_blocked_users_blocked_id (blocked_id)
);

-- ==========================================
-- 3. EMAIL VERIFICATION TOKENS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS email_verification_tokens (
    token_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    new_email STRING NOT NULL,
    token STRING UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    used_at TIMESTAMPTZ,
    INDEX idx_email_verification_tokens_token (token),
    INDEX idx_email_verification_tokens_user_id (user_id),
    INDEX idx_email_verification_tokens_expires_at (expires_at)
);

-- ==========================================
-- VERIFICATION QUERIES
-- ==========================================
-- Check tables created
SHOW TABLES;
