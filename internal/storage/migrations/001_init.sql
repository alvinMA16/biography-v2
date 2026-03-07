-- Biography V2 初始数据库结构
-- 使用 PostgreSQL

-- ============================================
-- 用户表
-- ============================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(20) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,

    -- 基本信息
    nickname VARCHAR(50),              -- 姓名
    preferred_name VARCHAR(50),        -- 称呼（用户希望被怎么称呼）
    gender VARCHAR(10),                -- 性别: 男/女
    birth_year INTEGER,                -- 出生年份
    hometown VARCHAR(100),             -- 家乡
    main_city VARCHAR(100),            -- 主要生活城市

    -- 状态
    profile_completed BOOLEAN DEFAULT FALSE,  -- 是否完成信息收集
    era_memories TEXT,                        -- 时代记忆（AI生成的背景知识）

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE  -- 软删除
);

CREATE UNIQUE INDEX idx_users_phone ON users(phone) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON users(deleted_at);

-- ============================================
-- 对话表
-- ============================================
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- 对话信息
    topic VARCHAR(200),           -- 话题标题
    greeting TEXT,                -- 开场白
    context TEXT,                 -- 预生成的上下文
    summary TEXT,                 -- 对话摘要（AI生成）

    -- 状态
    status VARCHAR(20) DEFAULT 'active',  -- active, completed, abandoned

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_conversations_user_id ON conversations(user_id);
CREATE INDEX idx_conversations_status ON conversations(status);
CREATE INDEX idx_conversations_created_at ON conversations(created_at DESC);

-- ============================================
-- 消息表
-- ============================================
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,

    -- 消息内容
    role VARCHAR(20) NOT NULL,    -- user, assistant
    content TEXT NOT NULL,

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);

-- ============================================
-- 回忆录表
-- ============================================
CREATE TABLE memoirs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,

    -- 内容
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,

    -- 时间线信息
    time_period VARCHAR(100),     -- 时间段描述（如"1970年代"）
    start_year INTEGER,           -- 开始年份
    end_year INTEGER,             -- 结束年份
    sort_order INTEGER DEFAULT 0, -- 排序顺序

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE  -- 软删除
);

CREATE INDEX idx_memoirs_user_id ON memoirs(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_memoirs_sort_order ON memoirs(user_id, sort_order);
CREATE INDEX idx_memoirs_deleted_at ON memoirs(deleted_at);

-- ============================================
-- 话题候选表
-- ============================================
CREATE TABLE topic_candidates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- 话题内容
    title VARCHAR(200) NOT NULL,
    greeting TEXT,                -- 开场白
    context TEXT,                 -- 背景上下文

    -- 状态
    status VARCHAR(20) DEFAULT 'pending',  -- pending, approved, rejected, used
    source VARCHAR(50) DEFAULT 'ai',       -- ai, manual

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_topic_candidates_user_id ON topic_candidates(user_id);
CREATE INDEX idx_topic_candidates_status ON topic_candidates(status);

-- ============================================
-- 激励语/问候语表
-- ============================================
CREATE TABLE quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- 内容
    content TEXT NOT NULL,
    type VARCHAR(20) NOT NULL,    -- motivational, greeting

    -- 配置
    is_active BOOLEAN DEFAULT TRUE,
    show_greeting BOOLEAN DEFAULT FALSE,  -- 是否在问候语中显示

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_quotes_type ON quotes(type) WHERE is_active = TRUE;

-- ============================================
-- 系统配置表
-- ============================================
CREATE TABLE system_configs (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    description VARCHAR(500),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 初始配置
INSERT INTO system_configs (key, value, description) VALUES
    ('default_llm_provider', 'gemini', '默认 LLM 提供者'),
    ('default_tts_voice', 'zh_male_shaonianzixin_brayan_v2_saturn_bigtts', '默认 TTS 音色'),
    ('topic_pool_size', '8', '话题池大小'),
    ('memoir_auto_generate', 'true', '是否自动生成回忆录');

-- ============================================
-- 更新时间触发器
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER conversations_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER memoirs_updated_at
    BEFORE UPDATE ON memoirs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER topic_candidates_updated_at
    BEFORE UPDATE ON topic_candidates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER quotes_updated_at
    BEFORE UPDATE ON quotes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
