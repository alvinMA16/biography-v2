package prompt

// TopicPrompt 话题生成 prompt
const TopicPrompt = `你是一位温暖的访谈专家，擅长引导老年人回忆人生故事。

请根据用户信息，生成 {{.Count}} 个适合聊天的话题。

## 用户信息
- 称呼：{{.UserName}}
- 出生年份：{{.BirthYear}}
- 家乡：{{.Hometown}}
- 主要生活城市：{{.MainCity}}

## 已有话题（避免重复）
{{.ExistingTopics}}

## 已有回忆录主题（可以延伸但不重复）
{{.ExistingMemoirs}}

## 要求
1. 话题要具体、有画面感，避免抽象空泛
2. 结合用户的年代背景和地域特色
3. 涵盖不同人生阶段：童年、青年、工作、家庭等
4. 语气亲切，像朋友聊天
5. 每个话题配一句开场白，自然引入
6. 为每个话题生成一小段时代背景提示 era_context，帮助后续聊天更自然地进入语境
7. era_context 要像聊天提示，不要写成百科介绍
8. era_context 最多 100 字，最多 2 句；如果这个话题不需要时代背景，就输出空字符串

## 输出格式
JSON 数组：
[
  {
    "title": "话题标题（简短）",
    "greeting": "开场白（一句话，亲切自然）",
    "context": "话题背景说明（帮助 AI 理解如何展开）",
    "era_context": "与该话题强相关的时代背景提示，不需要则为空字符串"
  }
]`

// TopicPromptQuick 快速话题生成（fewer tokens）
const TopicPromptQuick = `为{{.BirthYear}}年出生、来自{{.Hometown}}的老人生成{{.Count}}个聊天话题。

避免这些已有话题：{{.ExistingTopics}}

输出 JSON 数组：[{"title": "话题", "greeting": "开场白", "context": "背景", "era_context": "时代提示"}]`
