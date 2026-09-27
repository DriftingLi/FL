#!/bin/bash
# 由 GitHub Actions CD 自动生成 — 请勿手动编辑
# 生成时间: 2026-09-24 09:24:32 UTC

export DEPLOY_PATH=C:\\Users\\86155\\AppData\\Local\\Temp\\TestDeployEnvChainPreservesProvidedValues1259666104\\001

# ---- 部署默认值：唯一事实源 deploy/env.defaults（ADR-0047 §5 / spec #940 片四）----
source "$DEPLOY_PATH/deploy/env.defaults"

# ---- 让位：镜像引用由部署脚本按 registry + tag 现算并写进 .env ----
# compose 的取值优先级是 shell 环境 > .env。生成物里的 BACKEND_IMAGE/FRONTEND_IMAGE/
# LIBREOFFICE_IMAGE 是「本地镜像名」兜底值，若留在环境里会盖掉脚本算好的镜像引用，
# compose 就会去 Docker Hub 拉不存在的 forklift-backend:latest（2026-09-13 testing 冒烟实测踩到）。
unset BACKEND_IMAGE FRONTEND_IMAGE LIBREOFFICE_IMAGE

# ---- 覆盖段：secrets / 环境里确实提供了值的变量才写出（空值保留上面的默认）----
# 变量清单直接读生成物，workflow 里不再手抄一份默认值：
export DOMAIN=prod.example.com
export PG_VOLUME=/srv/ceph/pgdata

# ---- 其余变量（密钥类：空默认交给 compose / 后端）----
export IMAGE_TAG=latest
export IMAGE_TAG_BACKEND=''
export IMAGE_TAG_FRONTEND=''
export IMAGE_TAG_LIBREOFFICE=''
export IMAGE_BACKEND=''
export IMAGE_FRONTEND=''
export IMAGE_LIBREOFFICE=''
export SSL_FULLCHAIN=''
export SSL_PRIVKEY=''
export GITHUB_TOKEN=''
export REGISTRY=ghcr.io
export REGISTRY_PROXY=''
export REGISTRY_PROXY_BIND=127.0.0.1
export KEEP_IMAGES=3
export DATABASE_URL=''
export DB_PASSWORD=''
export SECRET_KEY=''
export JWT_SECRET_KEY=''
export AUTH_COOKIE_DOMAIN=''
export ADMIN_DEFAULT_PASSWORD=''
export TUTOR_DEFAULT_PASSWORD=''
export STUDENT_DEFAULT_PASSWORD=''
export SMTP_HOST=''
export SMTP_USERNAME=''
export SMTP_PASSWORD=''
export SMTP_FROM=''
export TENCENT_SMS_SECRET_ID=''
export TENCENT_SMS_SECRET_KEY=''
export TENCENT_SMS_SDK_APP_ID=''
export TENCENT_SMS_SIGN_NAME=''
export TENCENT_SMS_TEMPLATE_REGISTER=''
export TENCENT_SMS_TEMPLATE_LOGIN=''
export TENCENT_SMS_TEMPLATE_PASSWORD=''
export TENCENT_SMS_TEMPLATE_BIND_PHONE=''
export WECHAT_MINI_PROGRAM_APP_ID=''
export WECHAT_MINI_PROGRAM_APP_SECRET=''
export WECHAT_OPEN_PLATFORM_APP_ID=''
export WECHAT_OPEN_PLATFORM_APP_SECRET=''
export CORS_ORIGINS=''
export COMPOSE_FILE=docker-compose.prod.yml
export SKIP_MIGRATION=false
export REDIS_PASSWORD=''
export R2_ENDPOINT=''
export R2_ACCOUNT_ID=''
export R2_ACCESS_KEY_ID=''
export R2_SECRET_ACCESS_KEY=''
export R2_BUCKET=''
export R2_PUBLIC_DOMAIN=''
export SWAGGER_USER=''
export SWAGGER_PASS=''
export BACKUP_REMOTE_HOST=''
export BACKUP_REMOTE_DIR=''
export BACKUP_REMOTE_KEY=''
export SKIP_BACKUP=false
export COMPOSE_PROFILES=''
export HEALTH_CHECK_RETRIES=20
export UP_WAIT_TIMEOUT=60
export DEPLOY_CMD_TIMEOUT=600
