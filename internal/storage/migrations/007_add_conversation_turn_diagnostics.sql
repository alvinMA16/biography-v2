CREATE TABLE IF NOT EXISTS conversation_turn_diagnostics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    turn_index INTEGER NOT NULL,
    mode VARCHAR(32) NOT NULL,
    outcome VARCHAR(32) NOT NULL,
    user_text_source VARCHAR(32) NOT NULL DEFAULT 'empty',
    user_text_preview TEXT NOT NULL DEFAULT '',
    user_text_length INTEGER NOT NULL DEFAULT 0,
    assistant_preview TEXT NOT NULL DEFAULT '',
    assistant_length INTEGER NOT NULL DEFAULT 0,
    user_stop_at TIMESTAMP WITH TIME ZONE NOT NULL,
    asr_final_at TIMESTAMP WITH TIME ZONE,
    llm_started_at TIMESTAMP WITH TIME ZONE,
    llm_completed_at TIMESTAMP WITH TIME ZONE,
    tts_started_at TIMESTAMP WITH TIME ZONE,
    tts_first_chunk_at TIMESTAMP WITH TIME ZONE,
    tts_completed_at TIMESTAMP WITH TIME ZONE,
    done_sent_at TIMESTAMP WITH TIME ZONE,
    error_stage VARCHAR(32),
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_turn_diagnostics_conversation_turn
    ON conversation_turn_diagnostics(conversation_id, turn_index);

CREATE INDEX IF NOT EXISTS idx_turn_diagnostics_created_at
    ON conversation_turn_diagnostics(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_turn_diagnostics_user_created
    ON conversation_turn_diagnostics(user_id, created_at DESC);
