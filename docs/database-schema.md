# 数据库设计

## ER 图

```
┌─────────────────┐       ┌─────────────────────┐
│     users       │       │   conversations     │
├─────────────────┤       ├─────────────────────┤
│ id (PK)         │──┐    │ id (PK)             │
│ phone           │  │    │ user_id (FK)        │──┐
│ password_hash   │  │    │ topic               │  │
│ nickname        │  │    │ greeting            │  │
│ preferred_name  │  │    │ context             │  │
│ gender          │  │    │ summary             │  │
│ birth_year      │  │    │ status              │  │
│ hometown        │  │    │ created_at          │  │
│ main_city       │  │    │ updated_at          │  │
│ profile_completed│  │    └─────────────────────┘  │
│ era_memories    │  │              │               │
│ created_at      │  │              │               │
│ updated_at      │  │              ▼               │
│ deleted_at      │  │    ┌─────────────────────┐  │
└─────────────────┘  │    │     messages        │  │
         │           │    ├─────────────────────┤  │
         │           │    │ id (PK)             │  │
         │           │    │ conversation_id (FK)│◄─┤
         │           │    │ role                │  │
         │           │    │ content             │  │
         │           │    │ created_at          │  │
         │           │    └─────────────────────┘  │
         │           │                             │
         │           │    ┌─────────────────────┐  │
         │           └───►│     memoirs         │  │
         │                ├─────────────────────┤  │
         │                │ id (PK)             │  │
         │                │ user_id (FK)        │◄─┤
         │                │ conversation_id (FK)│◄─┘
         │                │ title               │
         │                │ content             │
         │                │ time_period         │
         │                │ start_year          │
         │                │ end_year            │
         │                │ sort_order          │
         │                │ created_at          │
         │                │ updated_at          │
         │                │ deleted_at          │
         │                └─────────────────────┘
         │
         │                ┌─────────────────────┐
         └───────────────►│  topic_candidates   │
                          ├─────────────────────┤
                          │ id (PK)             │
                          │ user_id (FK)        │
                          │ title               │
                          │ greeting            │
                          │ context             │
                          │ status              │
                          │ source              │
                          │ created_at          │
                          │ updated_at          │
                          └─────────────────────┘

┌─────────────────┐       ┌─────────────────────┐
│     quotes      │       │   system_configs    │
├─────────────────┤       ├─────────────────────┤
│ id (PK)         │       │ key (PK)            │
│ content         │       │ value               │
│ type            │       │ description         │
│ is_active       │       │ updated_at          │
│ show_greeting   │       └─────────────────────┘
│ created_at      │
│ updated_at      │
└─────────────────┘
```

## 表说明

### users - 用户表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| phone | VARCHAR(20) | 手机号（唯一） |
| password_hash | VARCHAR(255) | 密码哈希 |
| nickname | VARCHAR(50) | 姓名 |
| preferred_name | VARCHAR(50) | 称呼 |
| gender | VARCHAR(10) | 性别 |
| birth_year | INTEGER | 出生年份 |
| hometown | VARCHAR(100) | 家乡 |
| main_city | VARCHAR(100) | 主要居住城市 |
| profile_completed | BOOLEAN | 是否完成信息收集 |
| era_memories | TEXT | 时代记忆（AI生成） |
| deleted_at | TIMESTAMP | 软删除时间 |

### conversations - 对话表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| user_id | UUID | 用户ID (FK) |
| topic | VARCHAR(200) | 话题标题 |
| greeting | TEXT | 开场白 |
| context | TEXT | 对话上下文 |
| summary | TEXT | 对话摘要 |
| status | VARCHAR(20) | 状态: active/completed/abandoned |

### messages - 消息表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| conversation_id | UUID | 对话ID (FK) |
| role | VARCHAR(20) | 角色: user/assistant |
| content | TEXT | 消息内容 |

### memoirs - 回忆录表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| user_id | UUID | 用户ID (FK) |
| conversation_id | UUID | 来源对话ID (FK, 可空) |
| title | VARCHAR(200) | 标题 |
| content | TEXT | 正文内容 |
| time_period | VARCHAR(100) | 时间段描述 |
| start_year | INTEGER | 开始年份 |
| end_year | INTEGER | 结束年份 |
| sort_order | INTEGER | 排序顺序 |
| deleted_at | TIMESTAMP | 软删除时间 |

### topic_candidates - 话题候选表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| user_id | UUID | 用户ID (FK) |
| title | VARCHAR(200) | 话题标题 |
| greeting | TEXT | 开场白 |
| context | TEXT | 背景信息 |
| status | VARCHAR(20) | 状态: pending/approved/rejected/used |
| source | VARCHAR(50) | 来源: ai/manual |

### quotes - 激励语表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| content | TEXT | 内容 |
| type | VARCHAR(20) | 类型: motivational/greeting |
| is_active | BOOLEAN | 是否启用 |
| show_greeting | BOOLEAN | 是否显示问候语 |

### system_configs - 系统配置表

| 字段 | 类型 | 说明 |
|------|------|------|
| key | VARCHAR(100) | 配置键 (PK) |
| value | TEXT | 配置值 |
| description | VARCHAR(500) | 说明 |

## 索引

- `users.phone` - 唯一索引，用于登录
- `conversations.user_id` - 查询用户的对话列表
- `conversations.created_at DESC` - 按时间排序
- `messages.conversation_id` - 查询对话的消息
- `memoirs.user_id` - 查询用户的回忆录
- `topic_candidates.user_id` - 查询用户的话题候选

## 软删除

以下表支持软删除（通过 `deleted_at` 字段）：
- users
- memoirs

查询时需要添加 `WHERE deleted_at IS NULL` 条件。

## 级联删除

- 删除用户时，级联删除其所有数据（conversations, messages, memoirs, topic_candidates）
- 删除对话时，级联删除其所有消息
