# Google Gemini LLM

## 概述

Google Gemini 大语言模型，支持文本生成、多模态等能力。

## SDK 安装

```bash
pip install google-genai
# 或
go get github.com/google/generative-ai-go
```

## Python 使用示例

```python
from google import genai
from google.genai import types

def create_gemini_client(api_key: str, proxy: str = None):
    """创建 Gemini 客户端"""
    http_options = types.HttpOptions(timeout=30_000)

    if proxy:
        http_options.client_args = {"proxy": proxy}
        http_options.async_client_args = {"proxy": proxy}

    return genai.Client(
        vertexai=False,
        api_key=api_key,
        http_options=http_options
    )

# 使用示例
client = create_gemini_client(
    api_key="your-api-key",
    proxy="http://your-proxy:port"  # 可选
)

# 同步调用
response = client.models.generate_content(
    model="gemini-2.5-flash",
    contents="Hello, how are you?"
)
print(response.text)

# 流式调用
for chunk in client.models.generate_content_stream(
    model="gemini-2.5-flash",
    contents="Tell me a story"
):
    print(chunk.text, end="")
```

## Go 使用示例

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/google/generative-ai-go/genai"
    "google.golang.org/api/option"
)

func main() {
    ctx := context.Background()

    client, err := genai.NewClient(ctx, option.WithAPIKey("your-api-key"))
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    model := client.GenerativeModel("gemini-2.5-flash")

    resp, err := model.GenerateContent(ctx, genai.Text("Hello"))
    if err != nil {
        log.Fatal(err)
    }

    for _, part := range resp.Candidates[0].Content.Parts {
        fmt.Println(part)
    }
}
```

## 可用模型

| 模型 | 说明 |
|------|------|
| gemini-2.5-flash | 快速响应，适合一般任务 |
| gemini-2.0-flash-lite | 更快更轻量 |
| gemini-2.5-pro | 最强能力，适合复杂任务 |

## 配置参数

| 参数 | 说明 |
|------|------|
| api_key | API 密钥 |
| timeout | 请求超时时间 (毫秒) |
| proxy | 代理地址 (可选) |

## 代理配置

由于网络原因，国内访问可能需要配置代理：

```python
# HTTP 代理
proxy = "http://127.0.0.1:7890"

# SOCKS5 代理
proxy = "socks5://127.0.0.1:1080"
```

## 错误处理

```python
from google.api_core import exceptions

try:
    response = client.models.generate_content(...)
except exceptions.ResourceExhausted:
    # 配额用尽
    pass
except exceptions.InvalidArgument:
    # 参数错误
    pass
except exceptions.DeadlineExceeded:
    # 超时
    pass
```

## 注意事项

1. API Key 需要在 Google AI Studio 获取
2. 国内访问需要配置代理
3. 注意 Token 用量和配额限制
4. 建议设置合理的超时时间
