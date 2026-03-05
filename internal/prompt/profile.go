package prompt

// ProfileExtractionPrompt 用户信息提取 prompt
const ProfileExtractionPrompt = `从以下对话中提取用户的个人信息。

## 对话内容
{{.Conversation}}

## 要求
只提取用户明确提到的信息，不要推测。如果某项信息未提及，设为 null。

## 输出格式
JSON：
{
  "nickname": "姓名或称呼",
  "preferred_name": "希望被怎么称呼",
  "gender": "male/female/null",
  "birth_year": 出生年份数字或null,
  "hometown": "家乡/出生地",
  "main_city": "主要生活/工作的城市"
}`

// ProfileExtractionPromptSimple 简化版
const ProfileExtractionPromptSimple = `从对话提取用户信息，未提及的设为null：

{{.Conversation}}

输出JSON：{"nickname":null,"preferred_name":null,"gender":null,"birth_year":null,"hometown":null,"main_city":null}`
