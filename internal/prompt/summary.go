package prompt

// SummaryPrompt 对话摘要生成 prompt
const SummaryPrompt = `请为以下对话生成一段简洁的摘要（100-200字）。

## 对话主题
{{.Topic}}

## 对话内容
{{.Conversation}}

## 要求
1. 概括主要内容和关键信息
2. 保留重要的人名、地名、时间
3. 突出情感色彩和故事亮点
4. 语言简洁流畅

直接输出摘要文本，不要 JSON 格式。`

// SummaryPromptShort 短摘要（用于列表展示）
const SummaryPromptShort = `用一句话（30字以内）概括这段对话的主题：

{{.Conversation}}

直接输出摘要。`
