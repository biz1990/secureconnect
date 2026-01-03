-- SecureConnect Database Schema for CockroachDB
-- Based on docs/07-database-schema.md
-- Version: 1.0

-- ==========================================
-- DROP EXISTING TABLES (for clean re-init)
-- ==========================================
DROP TABLE IF EXISTS one_time_pre_keys CASCADE;
DROP TABLE IF EXISTS signed_pre_keys CASCADE;
DROP TABLE IF EXISTS identity_keys CASCADE;
DROP TABLE IF EXISTS files CASCADE;
DROP TABLE IF EXISTS conversation_participants CASCADE;
DROP TABLE IF EXISTS conversation_settings CASCADE;
DROP TABLE IF EXISTS conversations CASCADE;
DROP TABLE IF EXISTS contacts CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- ==========================================
-- 1. USERS TABLE
-- ==========================================
CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email STRING UNIQUE NOT NULL,
    username STRING UNIQUE NOT NULL,
    password_hash STRING NOT NULL,
    display_name STRING NOT NULL,
    avatar_url STRING,
    status STRING DEFAULT 'offline', -- online, offline, busy, away
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    
    -- Indexes for search
    INDEX idx_users_email (email),
    INDEX idx_users_username (username),
    INDEX idx_users_status (status),
    INDEX idx_users_created (created_at DESC)
);

-- ==========================================
-- 2. E2EE KEYS TABLES (Signal Protocol)
-- ==========================================

-- Identity Keys (Long-term Ed25519)
CREATE TABLE identity_keys (
    user_id UUID PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    public_key_ed25519 STRING NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Signed Pre-Keys (Medium-term X25519, rotated every 7 days)
CREATE TABLE signed_pre_keys (
    key_id INT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    public_key STRING NOT NULL,
    signature STRING NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, key_id),
    INDEX idx_signed_prekeys_user (user_id),
    INDEX idx_signed_prekeys_created (created_at DESC)
);

-- One-Time Pre-Keys (Single-use X25519)
CREATE TABLE one_time_pre_keys (
    key_id INT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    public_key STRING NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, key_id),
    INDEX idx_one_time_unused (user_id, used) WHERE used = FALSE
);

-- ==========================================
-- 3. CONTACTS TABLE
-- ==========================================
CREATE TABLE contacts (
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    contact_user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    status STRING DEFAULT 'pending', -- pending, accepted, blocked
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, contact_user_id),
    INDEX idx_contacts_user (user_id),
    INDEX idx_contacts_status (user_id, status)
);

-- ==========================================
-- 4. CONVERSATIONS TABLES
-- ==========================================

-- Conversation Metadata
CREATE TABLE conversations (
    conversation_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type STRING NOT NULL, -- direct, group
    name STRING, -- For group chats
    avatar_url STRING,
    created_by UUID REFERENCES users(user_id),
    created_at TIMESTAMPTZ DEFAULT now(),
    INDEX idx_conversations_created (created_at DESC)
);

-- Conversation Participants
CREATE TABLE conversation_participants (
    conversation_id UUID REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    role STRING DEFAULT 'member', -- admin, member
    joined_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (conversation_id, user_id),
    INDEX idx_participants_user (user_id),
    INDEX idx_participants_conv (conversation_id)
);

-- Conversation Settings (Hybrid E2EE Control)
CREATE TABLE conversation_settings (
    conversation_id UUID PRIMARY KEY REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    is_e2ee_enabled BOOLEAN DEFAULT TRUE, -- Default: E2EE ON
    ai_enabled BOOLEAN DEFAULT FALSE, -- Only when E2EE=false or Edge AI
    recording_enabled BOOLEAN DEFAULT FALSE,
    message_retention_days INT DEFAULT 30,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- ==========================================
-- 5. FILES TABLE (Storage Service)
-- ==========================================
CREATE TABLE files (
    file_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    file_name STRING NOT NULL,
    file_size BIGINT NOT NULL,
    content_type STRING,
    minio_object_key STRING NOT NULL,
    is_encrypted BOOLEAN DEFAULT FALSE, -- Client-side encryption
    encryption_metadata JSONB, -- Client encryption info
    status STRING DEFAULT 'uploading', -- uploading, completed, deleted
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    INDEX idx_files_user (user_id, created_at DESC),
    INDEX idx_files_status (status)
);

-- ==========================================
-- 6. SUBSCRIPTIONS TABLE (SaaS Billing)
-- ==========================================
CREATE TABLE subscriptions (
    subscription_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    plan_type STRING NOT NULL, -- free, pro, enterprise
    status STRING NOT NULL, -- active, canceled, past_due
    start_date TIMESTAMPTZ DEFAULT now(),
    end_date TIMESTAMPTZ,
    INDEX idx_subscriptions_user (user_id),
    INDEX idx_subscriptions_status (status)
);

-- ==========================================
-- GRANT PERMISSIONS (if needed)
-- ==========================================
-- For CockroachDB insecure mode, grants are not needed
-- But for production with users, add:
-- GRANT ALL ON DATABASE secureconnect_poc TO secureconnect_user;

-- ==========================================
-- INSERT SAMPLE DATA (for testing)
-- ==========================================
-- Sample user for testing
INSERT INTO users (user_id, email, username, password_hash, display_name, status) VALUES
    ('00000000-0000-0000-0000-000000000001', 'admin@secureconnect.com', 'admin', '$2a$12$LQv3c1yqBWVHxk','Admin User', 'online'),
    ('00000000-0000-0000-0000-000000000002', 'test@example.com', 'testuser', '$2a$12$LQv3c1yqBWVHxkQB', 'Test User', 'offline');

-- Sample conversation
INSERT INTO conversations (conversation_id, type, name, created_by) VALUES
    ('00000000-0000-0000-0000-000000000101', 'direct', NULL, '00000000-0000-0000-0000-000000000001');

-- Sample conversation settings
INSERT INTO conversation_settings (conversation_id, is_e2ee_enabled, ai_enabled) VALUES
    ('00000000-0000-0000-0000-000000000101', TRUE, FALSE);

-- ==========================================
-- VERIFICATION QUERIES
-- ==========================================
-- Check tables created
SHOW TABLES;

-- Check users
SELECT user_id, email, username, display_name, status FROM users;
