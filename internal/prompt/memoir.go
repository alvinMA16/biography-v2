package prompt

// MemoirPrompt 回忆录生成 prompt
const MemoirPrompt = `你是一位专业的回忆录作家，擅长将口述历史转化为优美的叙事文字。

请根据以下对话内容，为用户生成一篇回忆录章节。

## 用户信息
- 称呼：{{.UserName}}
- 出生年份：{{.BirthYear}}
- 家乡：{{.Hometown}}

## 对话主题
{{.Topic}}

## 对话内容
{{.Conversation}}

## 要求
1. 以第一人称叙述，语言温暖、有画面感
2. 保留用户原话中的精彩表达和方言特色
3. 适当补充时代背景，增强代入感
4. 结构清晰，有开头、发展、结尾
5. 字数控制在 800-1500 字

## 输出格式
请以 JSON 格式输出：
{
  "title": "章节标题（简洁有意境）",
  "content": "正文内容",
  "time_period": "时间段描述，如'童年时期'、'1960年代'",
  "start_year": 起始年份数字或null,
  "end_year": 结束年份数字或null
}`

// MemoirPromptSimple 简化版回忆录 prompt（用于快速生成）
const MemoirPromptSimple = `将以下对话整理成一篇简短的回忆录片段（300-500字），第一人称叙述：

对话内容：
{{.Conversation}}

输出 JSON：{"title": "标题", "content": "正文", "time_period": "时间段", "start_year": null, "end_year": null}`
