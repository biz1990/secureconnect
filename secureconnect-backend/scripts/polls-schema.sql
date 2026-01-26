-- SecureConnect Polls and Votes Schema
-- This file contains the SQL schema for polls and votes functionality
-- Version: 1.0

-- ==========================================
-- 1. POLLS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS polls (
    poll_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    creator_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    question STRING NOT NULL,
    poll_type STRING NOT NULL DEFAULT 'single', -- 'single' or 'multi'
    allow_vote_change BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ,
    is_closed BOOLEAN NOT NULL DEFAULT FALSE,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    
    -- Indexes
    INDEX idx_polls_conversation (conversation_id),
    INDEX idx_polls_creator (creator_id),
    INDEX idx_polls_expires_at (expires_at),
    INDEX idx_polls_created_at (created_at DESC),
    
    -- Constraints
    CONSTRAINT polls_type_check CHECK (poll_type IN ('single', 'multi'))
);

-- ==========================================
-- 2. POLL OPTIONS TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS poll_options (
    option_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL REFERENCES polls(poll_id) ON DELETE CASCADE,
    option_text STRING NOT NULL,
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    
    -- Indexes
    INDEX idx_poll_options_poll_id (poll_id),
    INDEX idx_poll_options_display_order (poll_id, display_order),
    
    -- Unique constraint to ensure option_text is unique within a poll
    CONSTRAINT poll_options_unique_text UNIQUE (poll_id, option_text)
);

-- ==========================================
-- 3. POLL VOTES TABLE
-- ==========================================
CREATE TABLE IF NOT EXISTS poll_votes (
    vote_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL REFERENCES polls(poll_id) ON DELETE CASCADE,
    option_id UUID NOT NULL REFERENCES poll_options(option_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    voted_at TIMESTAMPTZ DEFAULT now(),
    
    -- Indexes
    INDEX idx_poll_votes_poll_id (poll_id),
    INDEX idx_poll_votes_user_id (user_id),
    INDEX idx_poll_votes_option_id (option_id),
    INDEX idx_poll_votes_poll_user (poll_id, user_id),
    
    -- Unique constraint: a user can only vote once per option
    CONSTRAINT poll_votes_unique UNIQUE (poll_id, user_id, option_id)
);

-- ==========================================
-- 4. FUNCTIONS AND TRIGGERS
-- ==========================================

-- Function to update updated_at timestamp on polls table
CREATE OR REPLACE FUNCTION update_polls_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update updated_at
CREATE TRIGGER trigger_update_polls_updated_at
    BEFORE UPDATE ON polls
    FOR EACH ROW
    EXECUTE FUNCTION update_polls_updated_at();

-- ==========================================
-- 5. VIEWS FOR COMMON QUERIES
-- ==========================================

-- View for poll results with vote counts
CREATE OR REPLACE VIEW poll_results_view AS
SELECT 
    p.poll_id,
    p.conversation_id,
    p.creator_id,
    p.question,
    p.poll_type,
    p.allow_vote_change,
    p.expires_at,
    p.is_closed,
    p.closed_at,
    p.created_at,
    p.updated_at,
    COUNT(DISTINCT v.user_id) AS total_votes,
    COUNT(DISTINCT CASE WHEN v.vote_id IS NOT NULL THEN v.user_id END) AS total_voters
FROM polls p
LEFT JOIN poll_votes v ON p.poll_id = v.poll_id
GROUP BY p.poll_id, p.conversation_id, p.creator_id, p.question, p.poll_type, 
         p.allow_vote_change, p.expires_at, p.is_closed, p.closed_at, p.created_at, p.updated_at;

-- View for poll option results
CREATE OR REPLACE VIEW poll_option_results_view AS
SELECT 
    po.option_id,
    po.poll_id,
    po.option_text,
    po.display_order,
    COUNT(v.vote_id) AS vote_count,
    -- Calculate percentage relative to total votes for this poll
    CASE 
        WHEN (SELECT COUNT(*) FROM poll_votes WHERE poll_id = po.poll_id) > 0 
        THEN ROUND((COUNT(v.vote_id)::NUMERIC / (SELECT COUNT(*)::NUMERIC FROM poll_votes WHERE poll_id = po.poll_id)) * 100, 2)
        ELSE 0 
    END AS vote_percentage
FROM poll_options po
LEFT JOIN poll_votes v ON po.option_id = v.option_id
GROUP BY po.option_id, po.poll_id, po.option_text, po.display_order;

-- ==========================================
-- 6. HELPER FUNCTIONS
-- ==========================================

-- Function to check if a user has voted in a poll
CREATE OR REPLACE FUNCTION user_has_voted(poll_uuid UUID, user_uuid UUID)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS(
        SELECT 1 FROM poll_votes 
        WHERE poll_id = poll_uuid AND user_id = user_uuid
    );
END;
$$ LANGUAGE plpgsql;

-- Function to get user's votes in a poll
CREATE OR REPLACE FUNCTION get_user_votes(poll_uuid UUID, user_uuid UUID)
RETURNS TABLE(option_id UUID) AS $$
BEGIN
    RETURN QUERY
    SELECT v.option_id
    FROM poll_votes v
    WHERE v.poll_id = poll_uuid AND v.user_id = user_uuid;
END;
$$ LANGUAGE plpgsql;

-- Function to close a poll
CREATE OR REPLACE FUNCTION close_poll(poll_uuid UUID)
RETURNS VOID AS $$
BEGIN
    UPDATE polls
    SET is_closed = TRUE, closed_at = NOW()
    WHERE poll_id = poll_uuid;
END;
$$ LANGUAGE plpgsql;

-- ==========================================
-- 7. CLEANUP FUNCTIONS (for testing)
-- ==========================================

-- Function to drop all poll-related tables (use with caution)
CREATE OR REPLACE FUNCTION drop_poll_tables()
RETURNS VOID AS $$
BEGIN
    DROP VIEW IF EXISTS poll_option_results_view CASCADE;
    DROP VIEW IF EXISTS poll_results_view CASCADE;
    DROP TABLE IF EXISTS poll_votes CASCADE;
    DROP TABLE IF EXISTS poll_options CASCADE;
    DROP TABLE IF EXISTS polls CASCADE;
    DROP FUNCTION IF EXISTS update_polls_updated_at() CASCADE;
    DROP FUNCTION IF EXISTS user_has_voted(UUID, UUID) CASCADE;
    DROP FUNCTION IF EXISTS get_user_votes(UUID, UUID) CASCADE;
    DROP FUNCTION IF EXISTS close_poll(UUID) CASCADE;
END;
$$ LANGUAGE plpgsql;

-- ==========================================
-- VERIFICATION QUERIES
-- ==========================================
-- Check tables created
SHOW TABLES LIKE '%poll%';

-- Check views created
SHOW VIEWS LIKE '%poll%';
