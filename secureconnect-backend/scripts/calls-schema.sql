-- Video/Call tables for call logs

CREATE TABLE IF NOT EXISTS calls (
    call_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    caller_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    call_type STRING NOT NULL CHECK (call_type IN ('audio', 'video')),
    status STRING NOT NULL CHECK (status IN ('ringing', 'active', 'ended', 'missed', 'declined')),
    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    duration INT DEFAULT 0, -- in seconds
    recording_url STRING, -- URL to call recording if enabled
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS call_participants (
    call_id UUID NOT NULL REFERENCES calls(call_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    is_muted BOOLEAN DEFAULT FALSE,
    is_video_on BOOLEAN DEFAULT TRUE,
    PRIMARY KEY (call_id, user_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_calls_conversation ON calls(conversation_id);
CREATE INDEX IF NOT EXISTS idx_calls_caller ON calls(caller_id);
CREATE INDEX IF NOT EXISTS idx_calls_started_at ON calls(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_call_participants_user ON call_participants(user_id);
