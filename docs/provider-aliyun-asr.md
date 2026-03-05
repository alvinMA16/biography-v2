# 阿里云实时语音识别 (ASR)

## 概述

对长时间的语音数据流进行识别，适用于会议演讲、视频直播等长时间不间断识别的场景。

## 服务地址

| 访问类型 | URL |
|---------|-----|
| 上海 | `wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1` |
| 北京 | `wss://nls-gateway-cn-beijing.aliyuncs.com/ws/v1` |
| 深圳 | `wss://nls-gateway-cn-shenzhen.aliyuncs.com/ws/v1` |
| 就近接入 | `wss://nls-gateway.aliyuncs.com/ws/v1` |
| 内网(上海) | `ws://nls-gateway-cn-shanghai-internal.aliyuncs.com:80/ws/v1` |

## 音频要求

- **格式**: PCM、PCM编码的WAV、OGG封装的OPUS、OGG封装的SPEEX、AMR、MP3、AAC
- **声道**: 单声道 (mono)
- **采样位数**: 16 bit
- **采样率**: 8000 Hz、16000 Hz

## 鉴权

使用 Token 进行鉴权，Token 需要通过阿里云 API 获取。

## 请求参数

| 参数 | 类型 | 必选 | 说明 |
|------|------|------|------|
| appkey | String | 是 | 控制台创建的项目 Appkey |
| format | String | 否 | 音频格式: PCM、WAV、OPUS、SPEEX、AMR、MP3、AAC |
| sample_rate | Integer | 否 | 采样率，默认 16000 Hz |
| enable_intermediate_result | Boolean | 否 | 是否返回中间识别结果，默认 false |
| enable_punctuation_prediction | Boolean | 否 | 是否添加标点，默认 false |
| enable_inverse_text_normalization | Boolean | 否 | 中文数字转阿拉伯数字，默认 false |
| max_sentence_silence | Integer | 否 | 断句静音阈值 200ms~6000ms，默认 800ms |
| enable_words | Boolean | 否 | 是否返回词信息，默认 false |
| disfluency | Boolean | 否 | 过滤语气词（声音顺滑），默认 false |
| enable_semantic_sentence_detection | Boolean | 否 | 语义断句，默认 false |

## 交互流程

```
1. 建立 WebSocket 连接 (带 Token)
2. 发送开始识别请求
3. 循环发送音频数据
4. 接收识别结果:
   - SentenceBegin: 句子开始
   - TranscriptionResultChanged: 中间结果
   - SentenceEnd: 句子结束 (含最终结果)
5. 发送结束识别请求
```

## 响应事件

### SentenceBegin - 句子开始

```json
{
  "header": {
    "namespace": "SpeechTranscriber",
    "name": "SentenceBegin",
    "status": 20000000,
    "task_id": "xxx"
  },
  "payload": {
    "index": 1,
    "time": 0
  }
}
```

### TranscriptionResultChanged - 中间结果

```json
{
  "header": {
    "name": "TranscriptionResultChanged",
    "status": 20000000
  },
  "payload": {
    "index": 1,
    "time": 1835,
    "result": "北京的天",
    "confidence": 1.0,
    "words": [
      {"text": "北京", "startTime": 630, "endTime": 930},
      {"text": "的", "startTime": 930, "endTime": 1110},
      {"text": "天", "startTime": 1110, "endTime": 1140}
    ]
  }
}
```

### SentenceEnd - 句子结束

```json
{
  "header": {
    "name": "SentenceEnd",
    "status": 20000000
  },
  "payload": {
    "index": 1,
    "time": 1820,
    "begin_time": 0,
    "result": "北京的天气。",
    "confidence": 1.0,
    "words": [...],
    "emo_tag": "neutral",
    "emo_confidence": 0.931
  }
}
```

## 常用错误码

| 状态码 | 说明 | 解决方案 |
|--------|------|----------|
| 40000001 | Token 过期或无效 | 重新获取 Token |
| 40000004 | 连接空闲超时 (10s) | 保持发送音频数据 |
| 40000005 | 并发请求过多 | 升级商用版或购买并发包 |
| 41010101 | 不支持的采样率 | 使用 8000Hz 或 16000Hz |

## 支持的语言

- 中文普通话 (16k/8k)
- 英语 (16k/8k)
- 粤语 (16k/8k)
- 四川话、上海话等方言 (16k)
- 日语、韩语、西班牙语等 (16k)
