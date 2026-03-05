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

## 输出格式
JSON 数组：
[
  {
    "title": "话题标题（简短）",
    "greeting": "开场白（一句话，亲切自然）",
    "context": "话题背景说明（帮助 AI 理解如何展开）"
  }
]`

// TopicPromptQuick 快速话题生成（fewer tokens）
const TopicPromptQuick = `为{{.BirthYear}}年出生、来自{{.Hometown}}的老人生成{{.Count}}个聊天话题。

避免这些已有话题：{{.ExistingTopics}}

输出 JSON 数组：[{"title": "话题", "greeting": "开场白", "context": "背景"}]`
