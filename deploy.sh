#!/bin/bash
set -e

# Biography V2 部署脚本
# 用法: ./deploy.sh [命令]
# 命令: init | migrate | start | stop | restart | logs | backup

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR/deploy"
MIGRATIONS_DIR="$SCRIPT_DIR/internal/storage/migrations"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 .env 文件
check_env() {
    if [ ! -f "$DEPLOY_DIR/.env" ]; then
        log_error ".env 文件不存在"
        log_info "请先执行: cp .env.example deploy/.env && vim deploy/.env"
        exit 1
    fi
}

# 初始化（首次部署）
cmd_init() {
    log_info "=== 初始化部署 ==="

    check_env

    log_info "启动数据库..."
    cd "$DEPLOY_DIR"
    docker compose up -d db

    log_info "等待数据库就绪..."
    sleep 10

    # 检查数据库是否就绪
    for i in {1..30}; do
        if docker compose exec -T db pg_isready -U biography -d biography > /dev/null 2>&1; then
            log_info "数据库已就绪"
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "数据库启动超时"
            exit 1
        fi
        sleep 1
    done

    # 执行迁移
    cmd_migrate

    # 启动服务
    cmd_start

    log_info "=== 初始化完成 ==="
}

# 执行数据库迁移
cmd_migrate() {
    log_info "=== 执行数据库迁移 ==="

    cd "$DEPLOY_DIR"

    for sql in "$MIGRATIONS_DIR"/*.sql; do
        if [ -f "$sql" ]; then
            filename=$(basename "$sql")
            log_info "执行: $filename"
            docker compose exec -T db psql -U biography -d biography < "$sql" 2>/dev/null || true
        fi
    done

    log_info "迁移完成"
}

# 启动服务
cmd_start() {
    log_info "=== 启动服务 ==="

    check_env
    cd "$DEPLOY_DIR"

    docker compose build --no-cache backend
    docker compose up -d

    log_info "等待服务启动..."
    sleep 5

    # 健康检查
    for i in {1..30}; do
        if curl -s http://localhost:8000/health > /dev/null 2>&1; then
            log_info "服务已启动"
            docker compose ps
            echo ""
            log_info "健康检查:"
            curl -s http://localhost:8000/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8000/health
            return 0
        fi
        sleep 1
    done

    log_error "服务启动超时，请检查日志: docker compose logs backend"
    exit 1
}

# 停止服务
cmd_stop() {
    log_info "=== 停止服务 ==="
    cd "$DEPLOY_DIR"
    docker compose down
    log_info "服务已停止"
}

# 重启服务
cmd_restart() {
    log_info "=== 重启服务 ==="
    cd "$DEPLOY_DIR"
    docker compose restart backend

    sleep 3
    if curl -s http://localhost:8000/health > /dev/null 2>&1; then
        log_info "服务重启成功"
    else
        log_warn "服务可能仍在启动中，请稍后检查"
    fi
}

# 查看日志
cmd_logs() {
    cd "$DEPLOY_DIR"
    docker compose logs -f backend
}

# 备份数据库
cmd_backup() {
    log_info "=== 备份数据库 ==="

    BACKUP_FILE="backup_$(date +%Y%m%d_%H%M%S).sql"

    cd "$DEPLOY_DIR"
    docker compose exec -T db pg_dump -U biography biography > "$BACKUP_FILE"

    if [ -f "$BACKUP_FILE" ]; then
        log_info "备份完成: $DEPLOY_DIR/$BACKUP_FILE"
        ls -lh "$BACKUP_FILE"
    else
        log_error "备份失败"
        exit 1
    fi
}

# 更新部署
cmd_update() {
    log_info "=== 更新部署 ==="

    log_info "执行数据库迁移..."
    cmd_migrate

    log_info "重新构建并启动..."
    cd "$DEPLOY_DIR"
    docker compose build --no-cache backend
    docker compose up -d

    sleep 5
    if curl -s http://localhost:8000/health > /dev/null 2>&1; then
        log_info "更新完成"
        curl -s http://localhost:8000/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8000/health
    else
        log_warn "服务可能仍在启动中"
    fi
}

# 从远程拉取并更新
cmd_pull() {
    log_info "=== 拉取远程代码并更新 ==="

    log_info "拉取最新代码..."
    git pull origin main || git pull origin dev

    cmd_update
}

# 显示状态
cmd_status() {
    cd "$DEPLOY_DIR"
    docker compose ps
    echo ""
    log_info "健康检查:"
    curl -s http://localhost:8000/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8000/health || echo "服务未运行"
}

# 显示帮助
cmd_help() {
    echo "Biography V2 部署脚本"
    echo ""
    echo "用法: $0 <命令>"
    echo ""
    echo "命令:"
    echo "  init      首次部署（启动数据库、迁移、启动服务）"
    echo "  migrate   执行数据库迁移"
    echo "  start     启动服务"
    echo "  stop      停止服务"
    echo "  restart   重启后端服务（不重新构建）"
    echo "  update    重新构建并部署（用于本地代码修改）"
    echo "  pull      从远程拉取代码并更新部署"
    echo "  logs      查看后端日志"
    echo "  backup    备份数据库"
    echo "  status    查看服务状态"
    echo "  help      显示此帮助"
    echo ""
    echo "首次部署:"
    echo "  1. cp .env.example deploy/.env"
    echo "  2. vim deploy/.env  # 配置环境变量"
    echo "  3. ./deploy.sh init"
    echo ""
    echo "更新流程:"
    echo "  本地修改后: ./deploy.sh update"
    echo "  从远程更新: ./deploy.sh pull"
}

# 主入口
case "${1:-help}" in
    init)
        cmd_init
        ;;
    migrate)
        cmd_migrate
        ;;
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    update)
        cmd_update
        ;;
    pull)
        cmd_pull
        ;;
    logs)
        cmd_logs
        ;;
    backup)
        cmd_backup
        ;;
    status)
        cmd_status
        ;;
    help|--help|-h)
        cmd_help
        ;;
    *)
        log_error "未知命令: $1"
        cmd_help
        exit 1
        ;;
esac
