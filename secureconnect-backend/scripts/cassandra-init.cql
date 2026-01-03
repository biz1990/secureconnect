-- SecureConnect Cassandra Schema
-- Based on docs/07-database-schema.md
-- Version: 1.0
-- Execute with: cqlsh < cassandra-init.cql

-- ==========================================
-- 1. CREATE KEYSPACE
-- ==========================================
DROP KEYSPACE IF EXISTS secureconnect_ks;

CREATE KEYSPACE IF NOT EXISTS secureconnect_ks
  WITH replication = {
    'class': 'SimpleStrategy',
    'replication_factor': 1  -- For dev; use 3 for production
  }
  AND durable_writes = true;

USE secureconnect_ks;

-- ==========================================
-- 2. MESSAGES TABLE (Main Chat Storage)
-- ==========================================
-- Partitioned by (conversation_id, bucket) for sharding
-- Clustered by created_at DESC for reverse chronological order
-- Bucketing strategy prevents hot partitions
DROP TABLE IF EXISTS messages;

CREATE TABLE messages (
    conversation_id UUID,
    bucket INT,              -- YYYYMM format (e.g., 202312)
    message_id TIMEUUID,     -- Time-based UUID for ordering
    sender_id UUID,
    content TEXT,            -- Base64 ciphertext or plaintext
    is_encrypted BOOLEAN,    -- CRITICAL: Hybrid E2EE flag
    message_type TEXT,       -- text, image, video, file
    metadata MAP<TEXT, TEXT>, -- AI results or file metadata
    created_at TIMESTAMP,
    PRIMARY KEY ((conversation_id, bucket), created_at, message_id)
) WITH CLUSTERING ORDER BY (created_at DESC)
  AND default_time_to_live = 2592000  -- 30 days auto-delete
  AND compaction = {
    'class': 'TimeWindowCompactionStrategy',
    'compaction_window_size': 1,
    'compaction_window_unit': 'DAYS'
  }
  AND comment = 'Main messages table with bucketing for scalability';

-- ==========================================
-- 3. USER MESSAGES BY SENDER (Index Table)
-- ==========================================
-- Denormalized table for querying user's sent messages
-- Allows "Find all messages I sent" queries
DROP TABLE IF EXISTS user_messages_by_sender;

CREATE TABLE user_messages_by_sender (
    sender_id UUID,
    bucket INT,
    message_id TIMEUUID,
    conversation_id UUID,
    content TEXT,
    is_encrypted BOOLEAN,
    created_at TIMESTAMP,
    PRIMARY KEY ((sender_id, bucket), created_at, message_id)
) WITH CLUSTERING ORDER BY (created_at DESC)
  AND default_time_to_live = 2592000
  AND comment = 'Index table for sender message queries';

-- ==========================================
-- 4. CONVERSATION LIST VIEW (Materialized View Alternative)
-- ==========================================
-- Maintains last message and unread count per user
-- Updated when new message arrives
DROP TABLE IF EXISTS conversation_list_view;

CREATE TABLE conversation_list_view (
    user_id UUID,
    conversation_id UUID,
    updated_at TIMESTAMP,
    last_message_snippet TEXT,
    last_message_sender_id UUID,
    unread_count INT,
    PRIMARY KEY (user_id, updated_at, conversation_id)
) WITH CLUSTERING ORDER BY (updated_at DESC)
  AND comment = 'Fast lookup for conversation list per user';

-- ==========================================
-- 5. CALL LOGS TABLE (Video/Audio History)
-- ==========================================
DROP TABLE IF EXISTS call_logs;

CREATE TABLE call_logs (
    call_id UUID,
    bucket INT,              -- YYYYMM
    conversation_id UUID,
    initiator_id UUID,
    call_type TEXT,          -- audio, video
    status TEXT,             -- initiated, ongoing, completed, failed
    participants LIST<UUID>,
    duration_seconds INT,
    is_encrypted BOOLEAN,    -- E2EE for call
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    PRIMARY KEY ((conversation_id, bucket), started_at, call_id)
) WITH CLUSTERING ORDER BY (started_at DESC)
  AND default_time_to_live = 7776000  -- 90 days
  AND comment = 'Call history with bucketing';

-- ==========================================
-- 6. INSERT SAMPLE DATA (for testing)
-- ==========================================
-- Calculate current bucket (YYYYMM)
-- Example: 202312 for December 2023

-- Sample messages
INSERT INTO messages (
    conversation_id, 
    bucket, 
    message_id, 
    sender_id, 
    content, 
    is_encrypted, 
    message_type, 
    created_at
) VALUES (
    00000000-0000-0000-0000-000000000101,
    202401,  -- January 2024
    now(),
    00000000-0000-0000-0000-000000000001,
    'Hello, this is a test message!',
    false,
    'text',
    toTimestamp(now())
);

INSERT INTO messages (
    conversation_id, 
    bucket, 
    message_id, 
    sender_id, 
    content, 
    is_encrypted, 
    message_type, 
    created_at
) VALUES (
    00000000-0000-0000-0000-000000000101,
    202401,
    now(),
    00000000-0000-0000-0000-000000000002,
    'eyJlbmNyeXB0ZWQi...',  -- Base64 ciphertext
    true,
    'text',
    toTimestamp(now())
);

-- ==========================================
-- 7. VERIFICATION QUERIES
-- ==========================================
-- Show tables
DESCRIBE TABLES;

-- Query messages
SELECT conversation_id, bucket, message_id, sender_id, content, is_encrypted, created_at 
FROM messages 
WHERE conversation_id = 00000000-0000-0000-0000-000000000101 
  AND bucket = 202401
LIMIT 10;

-- Show keyspace details
DESCRIBE KEYSPACE secureconnect_ks;
