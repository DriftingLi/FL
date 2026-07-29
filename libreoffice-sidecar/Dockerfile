# ===== LibreOffice Sidecar 服务 =====
# 提供 PPT → PDF → PNG 转换的 HTTP API
# 与 backend 共享 /data/uploads volume
# 镜像大小约 500MB(LibreOffice + poppler-utils + fonts + python)

FROM python:3.12-alpine

# 安装 LibreOffice + poppler-utils + 中日韩字体
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

EXPOSE 8000

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8000/health || exit 1

# 用 gunicorn 启动(单进程,LibreOffice 不支持并发)
CMD ["gunicorn", "--bind", "0.0.0.0:8000", "--workers", "1", "--timeout", "180", "app:app"]
