# Biography V2 部署指南

## 系统要求

- Docker 20.10+
- Docker Compose 2.0+
- 至少 1GB 内存
- 10GB 磁盘空间

## 快速部署

### 1. 获取代码

```bash
git clone <repo-url> biography-v2
cd biography-v2
```

### 2. 配置环境变量

```bash
cd deploy
cp ../.env.example .env
vim .env
```

**必须配置的环境变量：**

| 变量 | 说明 | 示例 |
|-----|------|-----|
| `POSTGRES_PASSWORD` | 数据库密码 | `MyStr0ngP@ssw0rd` |
| `JWT_SECRET` | JWT 签名密钥（至少32位） | `your-32-char-secret-key-here!!` |
| `ADMIN_API_KEY` | 管理后台 API 密钥 | `admin-secret-key` |
| `GEMINI_API_KEY` | Gemini API 密钥 | `AIza...` |

**可选配置：**

| 变量 | 说明 | 默认值 |
|-----|------|-------|
| `LLM_PROVIDER_DEFAULT` | 默认 LLM 提供者 | `gemini` |
| `DASHSCOPE_API_KEY` | 通义千问 API 密钥 | - |
| `ALIYUN_ACCESS_KEY_ID` | 阿里云 AccessKey（ASR） | - |
| `ALIYUN_ACCESS_KEY_SECRET` | 阿里云 Secret（ASR） | - |
| `ALIYUN_ASR_APP_KEY` | 阿里云语音识别 AppKey | - |
| `DOUBAO_APP_ID` | 豆包 TTS AppID | - |
| `DOUBAO_ACCESS_KEY` | 豆包 TTS AccessKey | - |

### 3. 启动数据库

```bash
docker compose up -d db
```

等待数据库就绪（约 5-10 秒）：

```bash
docker compose logs db | grep "database system is ready"
```

### 4. 执行数据库迁移

```bash
# 依次执行迁移脚本
docker compose exec -T db psql -U biography -d biography < ../internal/storage/migrations/001_init.sql
docker compose exec -T db psql -U biography -d biography < ../internal/storage/migrations/002_add_preset_tables.sql
docker compose exec -T db psql -U biography -d biography < ../internal/storage/migrations/003_add_missing_fields.sql
```

### 5. 启动后端服务

```bash
docker compose up -d --build
```

### 6. 验证部署

```bash
# 检查容器状态
docker compose ps

# 健康检查
curl http://localhost:8000/health
```

预期输出：
```json
{"service":"biography-v2","status":"healthy"}
```

---

## Nginx 反向代理配置

### HTTPS 配置（推荐）

```nginx
server {
    listen 80;
    server_name api.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    # API 和 WebSocket
    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;

        # WebSocket 支持
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # 转发真实 IP
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 长连接
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }
}
```

配置完成后重载 Nginx：

```bash
nginx -t && nginx -s reload
```

---

## 常用运维命令

### 服务管理

```bash
# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f backend

# 重启服务
docker compose restart backend

# 停止所有服务
docker compose down

# 重新构建并启动
docker compose up -d --build
```

### 数据库操作

```bash
# 进入数据库
docker compose exec db psql -U biography -d biography

# 备份数据库
docker compose exec db pg_dump -U biography biography > backup_$(date +%Y%m%d).sql

# 恢复数据库
docker compose exec -T db psql -U biography -d biography < backup.sql
```

### 日志查看

```bash
# 查看后端日志（实时）
docker compose logs -f backend

# 查看最近 100 行日志
docker compose logs --tail=100 backend

# 查看数据库日志
docker compose logs db
```

---

## 更新部署

```bash
cd biography-v2

# 拉取最新代码
git pull origin main

# 执行新的迁移（如果有）
cd deploy
docker compose exec -T db psql -U biography -d biography < ../internal/storage/migrations/xxx_new_migration.sql

# 重新构建并启动
docker compose up -d --build

# 验证
curl http://localhost:8000/health
```

---

## API 端点一览

### 公开端点

| 方法 | 路径 | 说明 |
|-----|------|-----|
| GET | `/health` | 健康检查 |
| POST | `/api/auth/register` | 用户注册 |
| POST | `/api/auth/login` | 用户登录 |

### 用户端点（需要 JWT）

| 方法 | 路径 | 说明 |
|-----|------|-----|
| GET | `/api/user/profile` | 获取用户信息 |
| PUT | `/api/user/profile` | 更新用户信息 |
| PUT | `/api/user/password` | 修改密码 |
| POST | `/api/user/era-memories` | 生成时代记忆 |
| GET | `/api/user/era-memories` | 获取时代记忆状态 |
| GET | `/api/user/export` | 导出用户数据 |
| DELETE | `/api/user/account` | 注销账户 |
| GET | `/api/conversations` | 对话列表 |
| POST | `/api/conversations` | 创建对话 |
| GET | `/api/conversations/:id` | 对话详情 |
| GET | `/api/conversations/:id/messages` | 对话消息 |
| POST | `/api/conversations/:id/end` | 结束对话（同步） |
| POST | `/api/conversations/:id/end-quick` | 结束对话（异步） |
| GET | `/api/memoirs` | 回忆录列表 |
| GET | `/api/memoirs/:id` | 回忆录详情 |
| PUT | `/api/memoirs/:id` | 编辑回忆录 |
| DELETE | `/api/memoirs/:id` | 删除回忆录 |
| GET | `/api/topics` | 获取话题选项 |
| WS | `/api/realtime/dialog` | 实时对话 WebSocket |

### 管理端点（需要 X-Admin-Key）

| 方法 | 路径 | 说明 |
|-----|------|-----|
| GET | `/api/admin/users` | 用户列表 |
| GET | `/api/admin/users/:id` | 用户详情 |
| GET | `/api/admin/users/:id/stats` | 用户统计 |
| PUT | `/api/admin/users/:id` | 更新用户 |
| DELETE | `/api/admin/users/:id` | 删除用户 |
| POST | `/api/admin/users/:id/reset-password` | 重置密码 |
| POST | `/api/admin/users/:id/toggle-active` | 切换状态 |
| GET | `/api/admin/conversations` | 对话列表 |
| GET | `/api/admin/conversations/:id` | 对话详情 |
| GET | `/api/admin/memoirs` | 回忆录列表 |
| PUT | `/api/admin/memoirs/:id` | 更新回忆录 |
| DELETE | `/api/admin/memoirs/:id` | 删除回忆录 |
| POST | `/api/admin/memoirs/:id/regenerate` | 重新生成回忆录 |
| GET | `/api/admin/topics` | 话题列表 |
| POST | `/api/admin/topics` | 创建话题 |
| PUT | `/api/admin/topics/:id` | 更新话题 |
| DELETE | `/api/admin/topics/:id` | 删除话题 |
| GET | `/api/admin/quotes` | 激励语列表 |
| POST | `/api/admin/quotes` | 创建激励语 |
| PUT | `/api/admin/quotes/:id` | 更新激励语 |
| DELETE | `/api/admin/quotes/:id` | 删除激励语 |
| GET | `/api/admin/era-memories` | 时代记忆预设列表 |
| POST | `/api/admin/era-memories` | 创建时代记忆预设 |
| PUT | `/api/admin/era-memories/:id` | 更新时代记忆预设 |
| DELETE | `/api/admin/era-memories/:id` | 删除时代记忆预设 |
| GET | `/api/admin/preset-topics` | 预设话题列表 |
| POST | `/api/admin/preset-topics` | 创建预设话题 |
| PUT | `/api/admin/preset-topics/:id` | 更新预设话题 |
| DELETE | `/api/admin/preset-topics/:id` | 删除预设话题 |
| GET | `/api/admin/welcome-messages` | 欢迎语列表 |
| POST | `/api/admin/welcome-messages` | 创建欢迎语 |
| PUT | `/api/admin/welcome-messages/:id` | 更新欢迎语 |
| DELETE | `/api/admin/welcome-messages/:id` | 删除欢迎语 |
| GET | `/api/admin/logs` | 审计日志 |
| GET | `/api/admin/llm/providers` | LLM 提供者列表 |
| PUT | `/api/admin/llm/providers/primary` | 设置主 LLM |
| POST | `/api/admin/llm/providers/:provider/test` | 测试 LLM |
| GET | `/api/admin/tts/voices` | TTS 音色列表 |
| POST | `/api/admin/tts/test` | 测试 TTS |
| GET | `/api/admin/monitor/health` | 系统健康检查 |
| GET | `/api/admin/monitor/stats` | 系统统计 |

---

## 故障排查

### 服务无法启动

```bash
# 查看详细错误日志
docker compose logs backend

# 常见问题：
# 1. 数据库连接失败 - 检查 POSTGRES_PASSWORD 是否正确
# 2. 端口被占用 - 检查 8000 端口
# 3. 镜像构建失败 - 检查 Go 依赖
```

### 数据库连接失败

```bash
# 检查数据库是否运行
docker compose ps db

# 测试连接
docker compose exec db psql -U biography -d biography -c "SELECT 1"
```

### WebSocket 连接失败

1. 检查 Nginx 配置中的 WebSocket 头
2. 确认 `proxy_read_timeout` 足够长
3. 检查防火墙是否允许长连接

### LLM 调用失败

```bash
# 检查 Provider 状态
curl -H "X-Admin-Key: your-key" http://localhost:8000/api/admin/monitor/health
```

---

## 数据备份

### 自动备份脚本

创建 `/etc/cron.daily/biography-backup`：

```bash
#!/bin/bash
BACKUP_DIR=/data/backups/biography
DATE=$(date +%Y%m%d)

mkdir -p $BACKUP_DIR

cd /path/to/biography-v2/deploy
docker compose exec -T db pg_dump -U biography biography | gzip > $BACKUP_DIR/biography_$DATE.sql.gz

# 保留最近 7 天的备份
find $BACKUP_DIR -name "*.sql.gz" -mtime +7 -delete
```

```bash
chmod +x /etc/cron.daily/biography-backup
```

---

## 联系方式

如有问题，请联系开发团队。
