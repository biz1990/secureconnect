-- =============================================================================
-- Cassandra Schema for SecureConnect
-- =============================================================================
-- This schema defines the message storage and call logs for time-series data

-- Create keyspace
CREATE KEYSPACE IF NOT EXISTS secureconnect_ks
WITH replication = {
    'class': 'SimpleStrategy',
    'replication_factor': 1
}
AND durable_writes = true;

-- Use the keyspace
USE secureconnect_ks;

-- =============================================================================
-- MESSAGES TABLE (Time-Series)
-- =============================================================================
-- Stores all chat messages with encryption support
CREATE TABLE IF NOT EXISTS messages (
    conversation_id UUID,
    message_id UUID,
    sender_id UUID,
    receiver_id UUID,
    content TEXT,
    is_encrypted BOOLEAN,
    encryption_key_id UUID,
    message_type TEXT,          -- text, image, video, audio, file
    media_url TEXT,
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP,
    read_at TIMESTAMP,
    deleted_at TIMESTAMP,
    edited_at TIMESTAMP,
    reply_to_message_id UUID,
    metadata MAP<TEXT, TEXT>,   -- Additional metadata as key-value pairs
    PRIMARY KEY ((conversation_id), sent_at, message_id)
) WITH CLUSTERING ORDER BY (sent_at DESC, message_id DESC)
AND comment = 'Stores chat messages in time-series format'
AND compaction = {
    'class': 'TimeWindowCompactionStrategy',
    'compaction_window_unit': 'DAYS',
    'compaction_window_size': 1
}
AND gc_grace_seconds = 864000;  -- 10 days

-- Index for searching messages by sender
CREATE INDEX IF NOT EXISTS idx_messages_sender 
ON messages (sender_id);

-- Index for searching messages by message_id
CREATE INDEX IF NOT EXISTS idx_messages_message_id 
ON messages (message_id);

-- =============================================================================
-- CALL LOGS TABLE (Time-Series)
-- =============================================================================
-- Stores video/audio call session logs
CREATE TABLE IF NOT EXISTS call_logs (
    user_id UUID,
    call_id UUID,
    conversation_id UUID,
    call_type TEXT,             -- audio, video
    call_direction TEXT,        -- incoming, outgoing
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    duration INT,               -- in seconds
    status TEXT,                -- completed, missed, declined, failed
    quality_rating INT,         -- 1-5 stars
    PRIMARY KEY ((user_id), started_at, call_id)
) WITH CLUSTERING ORDER BY (started_at DESC, call_id DESC)
AND comment = 'Stores call history in time-series format'
AND compaction = {
    'class': 'TimeWindowCompactionStrategy',
    'compaction_window_unit': 'DAYS',
    'compaction_window_size': 7
}
AND gc_grace_seconds = 864000;  -- 10 days

-- Index for searching by call_id
CREATE INDEX IF NOT EXISTS idx_call_logs_call_id 
ON call_logs (call_id);

-- =============================================================================
-- MESSAGE REACTIONS TABLE
-- =============================================================================
-- Stores emoji reactions to messages
CREATE TABLE IF NOT EXISTS message_reactions (
    message_id UUID,
    user_id UUID,
    reaction TEXT,              -- emoji code
    reacted_at TIMESTAMP,
    PRIMARY KEY ((message_id), user_id)
) WITH comment = 'Stores message reactions';

-- =============================================================================
-- MESSAGE ATTACHMENTS TABLE
-- =============================================================================
-- Stores file attachments metadata
CREATE TABLE IF NOT EXISTS message_attachments (
    message_id UUID,
    attachment_id UUID,
    file_name TEXT,
    file_type TEXT,
    file_size BIGINT,
    minio_url TEXT,
    thumbnail_url TEXT,
    uploaded_at TIMESTAMP,
    PRIMARY KEY ((message_id), attachment_id)
) WITH comment = 'Stores message attachment metadata';

-- =============================================================================
-- USER ACTIVITY TABLE (Time-Series)
-- =============================================================================
-- Tracks user activity for analytics
CREATE TABLE IF NOT EXISTS user_activity (
    user_id UUID,
    activity_type TEXT,         -- login, logout, message_sent, call_made, etc.
    activity_time TIMESTAMP,
    metadata MAP<TEXT, TEXT>,
    PRIMARY KEY ((user_id), activity_time, activity_type)
) WITH CLUSTERING ORDER BY (activity_time DESC)
AND comment = 'Tracks user activity for analytics'
AND default_time_to_live = 2592000;  -- 30 days TTL

-- =============================================================================
-- COMPLETION MESSAGE
-- =============================================================================
-- Schema created successfully
