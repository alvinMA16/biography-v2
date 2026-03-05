# 豆包播客语音合成 (TTS)

## 概述

对送入的播客主题文本或链接进行分析，流式生成双人播客音频。支持断点重试。

## 服务地址

```
wss://openspeech.bytedance.com/api/v3/sami/podcasttts
```

## 鉴权 (Request Headers)

| Key | 说明 | 必须 | 示例 |
|-----|------|------|------|
| X-Api-App-Id | 火山引擎控制台的 APP ID | 是 | your-app-id |
| X-Api-Access-Key | 火山引擎控制台的 Access Token | 是 | your-access-key |
| X-Api-Resource-Id | 资源 ID | 是 | volc.service_type.10050 |
| X-Api-App-Key | 固定值 | 是 | aGjiRDfUWi |
| X-Api-Request-Id | 请求 ID (uuid) | 否 | uuid |

## 二进制协议

WebSocket 使用二进制协议传输，整数使用**大端**表示。

### 请求帧格式

| Byte | 说明 |
|------|------|
| 0 | Protocol version (0b0001) + Header size (0b0001) |
| 1 | Message type (0b0001) + Flags |
| 2 | Serialization (0b0001=JSON) + Compression (0b0001=gzip) |
| 3 | Reserved (0x00) |
| 4-7 | Event number |
| 8-11 | Session ID length |
| 12-... | Session ID |
| ... | Payload length + Payload |

## 请求参数 (Payload)

| 字段 | 类型 | 必须 | 说明 |
|------|------|------|------|
| action | number | 是 | 0=长文本生成, 3=对话文本直接合成, 4=prompt扩展 |
| input_text | string | 否 | 待合成文本 (action=0, 最长32k) |
| nlp_texts | []object | 否 | 对话文本列表 (action=3) |
| nlp_texts.text | string | - | 每轮文本 (单轮≤300字, 总长≤10000字) |
| nlp_texts.speaker | string | - | 发音人 |
| audio_config.format | string | 否 | 格式: pcm/mp3/ogg_opus/aac, 默认 pcm |
| audio_config.sample_rate | number | 否 | 采样率: 16000/24000/48000, 默认 24000 |
| audio_config.speech_rate | number | 否 | 语速: -50~100, 默认 0 |
| speaker_info.speakers | []string | 否 | 发音人列表 (2个) |
| use_head_music | bool | 否 | 开头音效, 默认 true |
| use_tail_music | bool | 否 | 结尾音效, 默认 false |

## 可选发音人

| 系列 | 发音人名称 |
|------|-----------|
| 黑猫侦探社咪仔 | zh_female_mizaitongxue_v2_saturn_bigtts |
| | zh_male_dayixiansheng_v2_saturn_bigtts |
| 刘飞和潇磊 | zh_male_liufei_v2_saturn_bigtts |
| | zh_male_xiaolei_v2_saturn_bigtts |

## 事件定义

| Event Code | 含义 |
|------------|------|
| 150 | SessionStarted - 会话开始 |
| 360 | PodcastRoundStart - 轮次开始 (含 speaker, text) |
| 361 | PodcastRoundResponse - 音频数据 |
| 362 | PodcastRoundEnd - 轮次结束 |
| 363 | PodcastEnd - 播客结束 |
| 152 | SessionFinished - 会话结束 |
| 154 | UsageResponse - 用量统计 |
| 2 | FinishConnection (上行) |
| 52 | ConnectionFinished (下行) |

## 请求示例

### action=3 对话文本直接合成

```json
{
  "input_id": "test_podcast",
  "action": 3,
  "use_head_music": false,
  "audio_config": {
    "format": "pcm",
    "sample_rate": 24000,
    "speech_rate": 0
  },
  "nlp_texts": [
    {
      "speaker": "zh_male_dayixiansheng_v2_saturn_bigtts",
      "text": "今天我们要聊的是火山引擎的一些重磅发布。"
    },
    {
      "speaker": "zh_female_mizaitongxue_v2_saturn_bigtts",
      "text": "来看看都有哪些亮点。"
    }
  ]
}
```

## 响应示例

### PodcastRoundStart

```json
{
  "text_type": "",
  "speaker": "zh_male_dayixiansheng_v2_saturn_bigtts",
  "round_id": 1,
  "text": "今天我们要聊的是..."
}
```

**特殊 round_id:**
- `-1`: 开头音乐
- `9999`: 结尾音乐

### PodcastRoundEnd

```json
{
  "audio_duration": 8.419333
}
```

## 错误码

| Code | 说明 |
|------|------|
| 20000000 | 成功 |
| 45000000 | 并发超限 |
| 55000000 | 服务端错误 |
| 50700000 | 内容审核过滤 / 文本超长 |

## 交互流程

```
1. 建立 WebSocket 连接 (带鉴权 Headers)
2. 发送 StartSession (含 payload)
3. 接收事件循环:
   - PodcastRoundStart (轮次开始, 含文本)
   - PodcastRoundResponse (音频数据) x N
   - PodcastRoundEnd (轮次结束)
   - ... 重复 ...
   - SessionFinished
4. 发送 FinishConnection
5. 接收 ConnectionFinished
6. 关闭连接
```
