# collector 独立 Python 服务（spec #2671 / 票 #2672）。
# 只装 PG 驱动；解析用标准库。入口为 batch CLI，daemon 增量模式是后续票。
FROM python:3.12-slim

WORKDIR /app

COPY collector/requirements.txt ./collector/requirements.txt
RUN python3 -m pip install --no-cache-dir -r collector/requirements.txt

COPY collector/ ./collector/

# 默认入口是 batch；实际调用由 compose command / docker compose run 覆盖。
ENTRYPOINT ["python3", "-m", "collector.cli"]
