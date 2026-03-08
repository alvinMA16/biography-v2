# 对话提质方案 V2（简化版）

## Summary

- 目标：在保持架构简单、可维护的前提下，提升“更懂用户、少重复、少 AI 腔、追问更具体”。
- 方案固定为：`单强模型 + 轻量 profile 演进 + 分层 context builder + prompt/few-shot 优化`。
- 明确不做：复杂记忆系统、双模型编排、流式生成、新增记忆表、前端多状态重构。
- 外部 API 不变；仅调整实时链路内部上下文装配和 profile 回写策略。

## Profile 维护策略

- `users` 表继续作为唯一 profile 真相源，不新增 profile 相关表。
- Profile 全部由人工服务维护，AI 只读取，不写入、不补空、不修正。
- 首版 profile 固定为以下稳定字段：
  - `name`
  - `gender`
  - `birth_year`
  - `hometown`
  - `main_city`（可空）
- 运行时称呼统一按规则生成：
  - 男性：`X先生`
  - 女性：`X女士`
  - 信息不足：`您`
- 不再使用 `preferred_name` 作为产品概念，也不允许用户自由输入称呼。
- 不进入 profile 的信息：
  - 职业经历
  - 家庭关系细节
  - 个性偏好
  - 单次故事里的动态事实
  - 这些信息只进入对话摘要，不进入 `users` 表
- `ProfileCompleted` 改为“人工资料是否齐全”的业务状态，不再和 AI 抽取绑定。
- 需要调整的实现点：
  - 停用 [flow/service.go](/Users/peizhengma/Documents/biography-v2/internal/service/flow/service.go) 中的 `ExtractProfile` / 自动回写链路
  - 保留 [user/service.go](/Users/peizhengma/Documents/biography-v2/internal/service/user/service.go) 作为唯一资料更新入口

## 首次体验对话与 Context 管理策略

- 原 `profile_collection` 重新定位为“首次体验对话”。
- 它的目标不是收集 profile，而是：
  - 让用户知道产品怎么使用
  - 让用户先体验一次真实对话
  - 在不增加复杂度的前提下，沉淀一些早期聊天线索
- 首次体验对话结束后仍然生成普通 `summary`，但不单独建存储、不单独建 memory 概念。
- 首次体验对话的 summary 与普通对话 summary 共用同一套存储和读取逻辑。
- 在后续 context 使用上：
  - 如果已有真实故事型对话摘要，优先使用那些
  - 首次体验对话摘要只在历史较少时作为低优先级兜底
  - 它默认不比后续普通对话摘要更重要

- 新增内部 `ContextBuilder`，统一生成 `ChatContextPacket`，字段固定为：
  - `current_user_turn`
  - `recent_turns`
  - `topic`
  - `core_profile`
  - `recent_summary`
  - `constraints`
- 回忆录场景下，用户单轮发言可能很长，因此实时链路区分两种文本形态：
  - `raw_turn`：原始完整文本，用于数据库存储、会后摘要、回忆录生成
  - `working_turn`：给实时聊天模型看的工作视图，用于当轮回复和短上下文管理
- 实时 prompt 的装配顺序固定为：
  1. 当前用户这句话
  2. 最近 6 条非 system 消息
  3. 当前话题 `title + context`
  4. Core Profile
  5. 最近 1 条已完成对话摘要
  6. 按需注入的 `era_note`
- 各层规则固定为：
  - `recent_turns`：只取当前会话最近 6 条非 system 消息，但历史长轮次使用压缩后的工作视图，不直接带原文
  - `topic.context`：最长 120 字，超出截断
  - `recent_summary`：只取最近一条 `status=completed AND summary 非空` 的会话摘要，排除当前会话，最长 80 字
  - 没有摘要就省略，不回退到全量历史消息
- 长单轮处理规则固定为：
  - 短轮次：直接使用原文作为 `working_turn`
  - 长轮次：转换成“摘要卡”，不使用“开头 + 结尾 + 锚点”的粗压缩方式
- `working_turn` 的摘要卡固定包含：
  - `last_focus`：用户最后落点的 1-2 句，尽量保留原文
  - `story_summary`：这一轮在讲什么，1 句
  - `facts`：人物 / 时间 / 地点 / 事件，最多 3-5 条
  - `followup_candidates`：最值得追问的 2-3 个点
- 设计原则固定为：
  - “最后落点”优先级高于“开头怎么起头”
  - 当前轮可以保留更多细节，历史轮必须压缩
  - 原文不丢，只是不直接全部进入实时 prompt
- `EraMemories` 不再在实时聊天阶段做动态筛选。
- 时代背景前移到“话题生成”阶段处理：
  - AI 生成话题时，直接生成一个与该话题强相关的 `era_context`
  - `era_context` 是给后续聊天模型使用的一小段加工后背景，不是裸的时代记忆
  - 实时聊天只消费该话题自带的 `era_context`
- `era_context` 约束固定为：
  - 最多 100 字
  - 最多 2 句
  - 必须是有助于聊天展开的提示，不是百科说明
  - 如果模型认为该话题不需要时代背景，则返回空字符串
- 长版 `EraMemories` 继续保留在 `users.era_memories`，主要用于话题生成、回忆录生成等非实时场景
- 结论：
  - 实时对话不再负责决定“这轮要不要带时代背景”
  - 时代背景作为话题的一部分，在生成阶段一次性加工完成

## 规则与大模型的边界

- 规则负责：
  - 选哪些上下文进入 prompt
  - 控制每类 context 的数量和长度
  - 已知 profile 不重复追问
  - 决定是否允许注入 `recent_summary`
  - 统一回复格式约束：语音场景下 `1-3 句`
- 大模型负责：
  - 判断用户这一句是在叙事、解释、表达情绪还是换话题
  - 判断这一轮最值得追问的是人物、事件、感受、时间还是地点
  - 决定是否自然承接 `recent_summary`
  - 具体措辞和口语感
- 原则固定为：
  - 规则不替模型决定“回什么内容”
  - 规则只决定“给模型看什么”和“哪些产品约束必须稳定执行”

## Prompt 与交互

- 重写 [realtime.go](/Users/peizhengma/Documents/biography-v2/internal/prompt/realtime.go)
  - 从“泛人格描述”改为“角色 + context packet + 输出禁令 + few-shot”
  - few-shot 固定 4 组：
    - 承接故事并追问细节
    - 先接情绪再追问
    - 利用最近摘要自然续聊
    - 在不相关场景下忽略 era 背景
- 输出约束固定为：
  - 1-3 句
  - 不复述用户原话
  - 不用模板化空共情
  - 优先具体追问，不做书面总结
- 前端保持现有 3 态：
  - `正在听`
  - `正在想`
  - `正在说`
- 不新增协议事件，不做流式改造

## Test Plan

- Profile 回写
  - AI 不会对 `name`、`gender`、`birth_year`、`hometown`、`main_city` 做任何自动写入
  - 管理员修改后，实时对话读取到的是最新人工资料
- ContextBuilder
  - 无历史摘要时，只使用当前会话 + topic + core profile
  - 有历史摘要时，只带最近 1 条，且长度受限
  - 如果话题有 `era_context`，则作为该话题背景的一部分注入
  - 如果话题没有 `era_context`，实时链路不再额外筛选时代记忆
  - 首次体验对话摘要在有正常故事对话摘要时不会优先被带入
- 对话质量
  - founder 用 10-15 组真实用户资料做旧版/新版盲聊
  - 重点看 4 项：
    - 是否更像认识这个人
    - 是否更少重复提问
    - 是否追问更具体
    - 是否更少被 era 背景干扰
- 通过标准
  - 主观试聊中，上述 4 项整体明显优于旧版
  - 不出现显著延迟上升
  - 不出现明显“把 era 背景强行带进每一轮”的现象
- 部署提醒
  - 本地完成开发后，部署到服务器前需要执行 migration：
    - `internal/storage/migrations/004_add_topic_era_context.sql`
  - 未执行该 migration 时，带 `era_context` 的话题生成功能无法正常落库

## Assumptions

- 第一阶段只做“更懂用户”和“context 更干净”，不做长期复杂记忆。
- Profile 只维护人工服务录入的稳定字段，不承担故事记忆功能。
- `EraMemories` 是话题生成阶段的辅助材料，不是实时聊天的常驻上下文。
- 首次体验对话只是一种早期轻量对话，不承载 profile 采集职责。
- 若后续单模型仍然不稳定，再讨论增加 planner；当前阶段不引入第二个模型。

## 分步执行计划

1. 方案文档落盘
- 目标：把当前已确认方案整理成 `docs/chat-quality-plan-v2.md`
- 完成判定：文件创建成功，内容与当前方案一致

2. 停用 AI Profile 写入
- 目标：把 profile 收口为人工维护的权威资料，完全停止 AI 自动写入
- 代码范围：
  - [flow/service.go](/Users/peizhengma/Documents/biography-v2/internal/service/flow/service.go)
  - [user/service.go](/Users/peizhengma/Documents/biography-v2/internal/service/user/service.go)
- 完成判定：
  - 对话结束后不会再触发 profile 自动抽取和回写
  - 用户资料只能通过人工后台更新

3. 引入第一版 `ContextBuilder`
- 目标：把实时聊天的上下文装配从“直接拼 prompt”改成“先组包、再渲染 prompt”
- 第一版 `ChatContextPacket` 固定只包含：
  - `current_user_turn`
  - `recent_turns`
  - `topic`
  - `core_profile`
  - `recent_summary`
  - `constraints`
- 长单轮在 `ChatContextPacket` 中不直接带全文，而是带 `working_turn` 摘要卡；原文继续存储但不直接进入实时 prompt
- 完成判定：
  - 实时 prompt 不再直接依赖散落字段
  - 上下文来源和优先级变得可读、可控
  - 长单轮不会直接撑爆实时上下文，但原始内容仍保留给会后流程使用

4. 重写实时 prompt 与 few-shot
- 目标：让单模型在更干净的上下文里，输出更像真人、更具体的追问
- 代码范围：
  - [realtime.go](/Users/peizhengma/Documents/biography-v2/internal/prompt/realtime.go)
- 完成判定：
  - prompt 结构更短、更明确
  - 不再依赖大量泛规则堆砌

5. 话题生成时直接产出 `era_context`
- 目标：把时代背景选择从实时聊天阶段前移到话题生成阶段
- 代码范围：
  - [entity.go](/Users/peizhengma/Documents/biography-v2/internal/domain/topic/entity.go)
  - [topic.go](/Users/peizhengma/Documents/biography-v2/internal/prompt/topic.go)
  - [service.go](/Users/peizhengma/Documents/biography-v2/internal/service/llm/service.go)
  - [service.go](/Users/peizhengma/Documents/biography-v2/internal/service/topic/service.go)
  - [handler.go](/Users/peizhengma/Documents/biography-v2/internal/api/realtime/handler.go)
  - [protocol.go](/Users/peizhengma/Documents/biography-v2/internal/api/realtime/protocol.go)
  - [session.go](/Users/peizhengma/Documents/biography-v2/internal/api/realtime/session.go)
- 完成判定：
  - AI 生成话题时可同时生成 `era_context`
  - 实时聊天只消费话题自带的 `era_context`
  - 不再需要在实时链路里动态筛选时代记忆

6. 最低限度测试与回归验证
- 目标：为这次改造补最小可回归保障
- 固定补的测试类型：
  - profile 补空不覆盖
  - recent summary 选择规则
  - era note 触发与截断规则
- 完成判定：
  - 至少有最基本的行为回归保护
  - 手工试聊能验证 context 装配符合预期

7. Founder 试聊与下一轮决策
- 目标：确认这套简化方案是否真的提升体验，而不是只是“架构更漂亮”
- 试聊维度固定为：
  - 是否更像认识这个人
  - 是否更少重复提问
  - 是否追问更具体
  - 是否更少被 era 背景干扰
