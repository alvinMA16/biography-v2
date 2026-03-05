-- 补充缺失的字段和表

-- ============================================
-- 用户表补充字段
-- ============================================
ALTER TABLE users ADD COLUMN IF NOT EXISTS settings JSONB DEFAULT '{}';
-- settings 存储用户偏好：{"perspective": "first_person", "topic_preference": "..."}

-- ============================================
-- 对话表补充字段
-- ============================================
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS title VARCHAR(200);
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS topics JSONB DEFAULT '[]';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_conversations_deleted_at ON conversations(deleted_at) WHERE deleted_at IS NULL;

-- ============================================
-- 回忆录表补充字段
-- ============================================
ALTER TABLE memoirs ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'completed';
-- status: generating(生成中) / completed(已完成)
ALTER TABLE memoirs ADD COLUMN IF NOT EXISTS source_conversations JSONB DEFAULT '[]';
-- source_conversations: 来源对话ID数组

CREATE INDEX IF NOT EXISTS idx_memoirs_status ON memoirs(status);

-- ============================================
-- 欢迎语表（激励语/开屏语）
-- ============================================
CREATE TABLE IF NOT EXISTS welcome_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- 内容
    content TEXT NOT NULL,            -- 欢迎语内容
    show_greeting BOOLEAN DEFAULT TRUE, -- 是否显示在开屏页

    -- 状态
    is_active BOOLEAN DEFAULT TRUE,   -- 是否启用
    sort_order INTEGER DEFAULT 0,     -- 排序权重

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_welcome_messages_active ON welcome_messages(is_active) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_welcome_messages_sort ON welcome_messages(sort_order);

-- ============================================
-- 审计日志表
-- ============================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- 操作信息
    admin_id UUID,                    -- 操作者ID（可为空，系统操作）
    action VARCHAR(50) NOT NULL,      -- 操作类型：create_user, edit_user, reset_password, delete_user, etc.

    -- 目标信息
    target_type VARCHAR(50),          -- 目标类型：user, conversation, memoir, topic, etc.
    target_id UUID,                   -- 目标ID
    target_label VARCHAR(200),        -- 目标标签（如用户手机号、话题名称等）

    -- 详情
    detail JSONB,                     -- 操作详情（变更前后的数据等）
    ip_address VARCHAR(45),           -- 操作者IP

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_admin ON audit_logs(admin_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC);

-- ============================================
-- 更新时间触发器
-- ============================================
CREATE OR REPLACE TRIGGER welcome_messages_updated_at
    BEFORE UPDATE ON welcome_messages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
