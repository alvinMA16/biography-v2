# Biography V2

AI 驱动的人生故事记录应用 - 帮助老年人通过语音对话记录人生故事，生成书面回忆录。

## 技术栈

- **后端**: Go (Gin + pgx)
- **前端**: Vue/React (TBD)
- **数据库**: PostgreSQL
- **语音服务**:
  - ASR: 阿里云语音识别
  - TTS: 豆包播客接口
  - LLM: Gemini (可切换 DashScope/OpenAI)

## 项目结构

```
biography-v2/
├── cmd/server/          # 入口
├── internal/
│   ├── api/             # HTTP/WebSocket handlers
│   ├── domain/          # 业务逻辑
│   ├── provider/        # 外部服务抽象 (LLM/ASR/TTS)
│   └── storage/         # 数据存储
├── web/                 # 前端
├── deploy/              # 部署配置
└── docs/                # 文档
```

## 快速开始

### 本地开发

```bash
# 安装依赖
go mod download

# 复制环境变量
cp .env.example .env
# 编辑 .env 填入 API keys

# 启动数据库
docker compose -f deploy/docker-compose.yml up db -d

# 运行服务
go run ./cmd/server
```

### Docker 部署

```bash
cd deploy
cp ../.env.example .env
# 编辑 .env
docker compose up -d --build
```

## 环境变量

参见 `.env.example`

## API 文档

TBD

## License

Private
