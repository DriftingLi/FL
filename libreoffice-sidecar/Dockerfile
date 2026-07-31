# ===== LibreOffice Sidecar 服务 =====
# 提供两类 HTTP API：
#   1. PPT → PDF → WebP 转换（/convert）
#   2. 任意图片 → WebP 压缩（/convert-image）
# 与 backend 不再共享 volume，全部通过 HTTP multipart 收发
# 镜像大小约 500MB(LibreOffice + poppler-utils + fonts + python + Pillow)

FROM python:3.12-alpine

# 安装 LibreOffice + poppler-utils + 中日韩字体
# Pillow 10.4.0 在 alpine 上提供 musllinux wheel，无需额外编译依赖
RUN apk add --no-cache \
        libreoffice \
        poppler-utils \
        font-noto-cjk && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple

COPY app.py .

# 创建非 root 用户 + LibreOffice 所需的家目录（~/.config 用于 LibreOffice 配置缓存）
# gunicorn 监听 8000(>1024)，非 root 可绑定
RUN adduser -D -s /bin/sh -h /home/app app && \
    mkdir -p /home/app/.config && \
    chown -R app:app /app /home/app

EXPOSE 8000

USER app

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8000/health || exit 1

# 用 gunicorn 启动(单进程,LibreOffice 不支持并发)
CMD ["gunicorn", "--bind", "0.0.0.0:8000", "--workers", "1", "--timeout", "180", "app:app"]
