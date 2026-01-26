-- =============================================================================
-- NOTIFICATIONS SCHEMA
-- CockroachDB Schema for SecureConnect Notifications
-- =============================================================================

-- Notifications Table
CREATE TABLE IF NOT EXISTS notifications (
    notification_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('message', 'call', 'friend_request', 'system')),
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    data JSONB,
    is_read BOOLEAN NOT NULL DEFAULT false,
    is_pushed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at TIMESTAMPTZ,

    CONSTRAINT fk_notifications_user FOREIGN KEY (user_id)
        REFERENCES users(user_id) ON DELETE CASCADE
);

-- Indexes for efficient queries
CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_user_read ON notifications(user_id, is_read, created_at DESC);
CREATE INDEX idx_notifications_unpushed ON notifications(is_pushed, created_at) WHERE is_pushed = false;

-- Notification Preferences Table
CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id UUID PRIMARY KEY,
    email_enabled BOOLEAN NOT NULL DEFAULT true,
    push_enabled BOOLEAN NOT NULL DEFAULT true,
    message_enabled BOOLEAN NOT NULL DEFAULT true,
    call_enabled BOOLEAN NOT NULL DEFAULT true,
    friend_request_enabled BOOLEAN NOT NULL DEFAULT true,
    system_enabled BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_notification_preferences_user FOREIGN KEY (user_id)
        REFERENCES users(user_id) ON DELETE CASCADE
);

-- Insert default preferences for existing users (run this once after table creation)
INSERT INTO notification_preferences (user_id)
SELECT user_id FROM users
ON CONFLICT (user_id) DO NOTHING;
