"""真 PG 契约测试的共享脚手架（#2678 审阅修复：消除三份逐字节相同的 setUpClass）。

三个 e2e 测试文件（test_e2e_contract / test_backfill_e2e / test_daemon_e2e）此前各自
手抄同一段：建随机 schema、按序灌 migrations、建 original_prices / ai_configs /
ai_feature_bindings 最小表、塞加密 api_key。迁移号加一个就要改 N 处——抽到这里。

纪律沿用 backend/internal/testutil/pg.go：
  - 无 DATABASE_URL ⇒ 调用方 skip（本模块不自己 skip，避免掩盖）；
  - 随机 schema（`<prefix>_<hex>`），tearDown DROP SCHEMA CASCADE；
  - 一切查询按 current_schema() 收窄，不碰 public。
"""

from __future__ import annotations

import secrets
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURES = REPO_ROOT / "collector" / "fixtures"
MIGRATION_DIR = REPO_ROOT / "backend" / "migrations"

# 与 backend/internal/security/crypto.go 的 EncryptSecret 对齐：
# EncryptSecret("sk-test-vector-2673", "unit-test-master-key") 的产物。
ENCRYPTED_KEY = "enc:v1:ULpTLw09HcsKToqJbwY8777i33KFU0T7i1Rh1r3O82ub5SKMjvT9ODrltwB5YIs="
MASTER_KEY = "unit-test-master-key"
DECRYPTED_KEY = "sk-test-vector-2673"
FEATURE_KEY = "market_sample_extract"

def create_schema(prefix: str = "ct"):
    """建随机 schema 并返回 (conn, schema, sql_module)。调用方负责 tearDown。"""
    import psycopg
    from psycopg import sql

    schema = f"{prefix}_" + secrets.token_hex(4)
    conn = psycopg.connect(DATABASE_URL_FROM_ENV())
    conn.execute(sql.SQL("CREATE SCHEMA {}").format(sql.Identifier(schema)))
    conn.commit()
    conn.execute(sql.SQL("SET search_path TO {}").format(sql.Identifier(schema)))
    return conn, schema, sql


def DATABASE_URL_FROM_ENV() -> str:
    import os

    return os.environ.get("DATABASE_URL", "")


def apply_migrations(conn) -> None:
    """在随机 schema 执行实际迁移。"""
    for migration in sorted(MIGRATION_DIR.glob("*.up.sql")):
        conn.execute(migration.read_text(encoding="utf-8"))
    conn.commit()


def seed_test_data(conn) -> None:
    """使用真实表结构，仅注入测试自己的原价与 AI 配置。"""
    conn.execute("DELETE FROM original_prices")
    conn.execute(
        "INSERT INTO ai_configs (name, api_key, base_url, model) VALUES (%s, %s, %s, %s)",
        ("collector-stub", ENCRYPTED_KEY, "http://stub.invalid/v1", "stub-model"),
    )
    conn.execute(
        "INSERT INTO ai_feature_bindings (feature_key, config_id) "
        "SELECT %s, id FROM ai_configs WHERE name = %s",
        (FEATURE_KEY, "collector-stub"),
    )
    conn.commit()


def drop_schema(conn, schema: str, sql_module) -> None:
    conn.execute(
        sql_module.SQL("DROP SCHEMA {} CASCADE").format(sql_module.Identifier(schema))
    )
    conn.commit()
    conn.close()


def fixture_payload(name: str) -> str:
    return (FIXTURES / name).read_text(encoding="utf-8")
