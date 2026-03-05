-- 补充预设数据表和用户字段

-- ============================================
-- 用户表补充字段
-- ============================================
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS era_memories_status VARCHAR(20) DEFAULT 'none';
-- era_memories_status: none(未收集基础信息) / pending(等待生成) / generating(生成中) / completed(已完成) / failed(失败)

-- ============================================
-- 时代记忆预设表（全局历史事件库）
-- ============================================
CREATE TABLE IF NOT EXISTS era_memories_preset (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- 时间范围
    start_year INTEGER NOT NULL,      -- 事件开始年份
    end_year INTEGER NOT NULL,        -- 事件结束年份（可等于 start_year）

    -- 内容
    category VARCHAR(50),             -- 分类：历史事件/流行文化/社会风潮/品牌零食等
    content TEXT NOT NULL,            -- 事件/现象描述

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_era_memories_preset_years ON era_memories_preset(start_year, end_year);
CREATE INDEX IF NOT EXISTS idx_era_memories_preset_category ON era_memories_preset(category);

-- ============================================
-- 预设话题表（全局共享，新用户首次对话时使用）
-- ============================================
CREATE TABLE IF NOT EXISTS preset_topics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- 话题内容
    topic VARCHAR(200) NOT NULL,      -- 话题名称
    greeting TEXT NOT NULL,           -- 开场白
    chat_context TEXT,                -- 对话上下文（追问路径等）

    -- 适用年龄段
    age_start INTEGER,                -- 话题对应的人生阶段起始年龄
    age_end INTEGER,                  -- 话题对应的人生阶段结束年龄

    -- 状态
    is_active BOOLEAN DEFAULT TRUE,   -- 是否启用
    sort_order INTEGER DEFAULT 0,     -- 排序权重

    -- 时间戳
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_preset_topics_active ON preset_topics(is_active) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_preset_topics_sort ON preset_topics(sort_order);

-- ============================================
-- 更新时间触发器
-- ============================================
CREATE TRIGGER IF NOT EXISTS era_memories_preset_updated_at
    BEFORE UPDATE ON era_memories_preset
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER IF NOT EXISTS preset_topics_updated_at
    BEFORE UPDATE ON preset_topics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
