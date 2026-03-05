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

// ProfileCompletionCheckPrompt 信息收集完成度检查
const ProfileCompletionCheckPrompt = `请判断以下对话是否应该结束信息收集环节。

## 需要收集的 4 项信息
1. 称呼 - 用户希望被怎么称呼（如"老张"、"张爷爷"等），或者用户表示没有特别偏好（如"怎么叫都行"、直接说了自己姓名）也算确认
2. 出生年份 - 明确的出生年份，或者能推算出年份的年龄
3. 家乡 - 出生地或老家
4. 生活时间最长的城市

## 对话内容
{{.Conversation}}

## 判断标准（满足任一即 complete: true）
1. 4 项信息都已经在对话中被提及并确认
2. 记录师的最后一条消息明显是在结束对话（如道别、说"很高兴认识您"、表示可以开始聊故事了等）
- 用户对某项信息表示"不想说"或"跳过"也算已处理，该项视为已确认

## 输出格式（仅输出 JSON，不要其他内容）
{"complete": true或false}`
