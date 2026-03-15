ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS mode VARCHAR(20) NOT NULL DEFAULT 'normal';

CREATE INDEX IF NOT EXISTS idx_conversations_mode ON conversations(mode);
