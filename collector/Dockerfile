# collector 独立 Python 服务（spec #2671 / 票 #2672）。
# 只装 PG 驱动 + cryptography（解密 ai_configs.api_key）；解析用标准库。
# 入口是 CLI，同一镜像承载全部模式（batch / backfill / daemon / reclean / rematch /
# unmatched）——#2678 起 daemon 增量模式可与 batch 共用同一镜像，无需另一个镜像。
FROM python:3.12-slim

WORKDIR /app

COPY collector/requirements.txt ./collector/requirements.txt
RUN python3 -m pip install --no-cache-dir -r collector/requirements.txt

COPY collector/ ./collector/

# 默认入口是 CLI；实际调用由 compose command / docker compose run 覆盖。
# 生产 daemon 由 compose 服务传 `daemon --list-url ...` 常驻。
ENTRYPOINT ["python3", "-m", "collector.cli"]
