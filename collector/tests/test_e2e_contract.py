"""collector batch 端到端契约测试（spec #2671 指定的唯一 seam：fixture 进 → 行出）。

纪律（与 backend/internal/testutil/pg.go 同源）：
  - 本机无 DATABASE_URL ⇒ 整个类干净 skip，首跑在 CI（collector-test job 提供真 PG）；
  - 随机 schema（ct_<hex>）隔离，迁移只落在本 schema；
  - 查询按 current_schema() 收窄，并显式断言 current_schema() == 本用例 schema；
  - tearDownClass DROP SCHEMA CASCADE 清理。

psycopg 在 skip 判定之后才 import：本机既无 DATABASE_URL 也无驱动时不会因缺依赖报错。
"""

import os
import secrets
import unittest
from pathlib import Path

DATABASE_URL = os.environ.get("DATABASE_URL")

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURE = REPO_ROOT / "collector" / "fixtures" / "rmfysszc_announcement.html"
MIGRATION_DIR = REPO_ROOT / "backend" / "migrations"

EXPECTED_SOURCE_URL = "https://www.rmfysszc.com/auction/notice/2026/11001.html"
EXPECTED_TITLE = "杭叉 CPCD30 叉车公开拍卖公告"


@unittest.skipUnless(DATABASE_URL, "本机无 DATABASE_URL：真 PG 契约测试跳过（首跑在 CI）")
class CollectorBatchE2EContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        import psycopg
        from psycopg import sql

        cls.psycopg = psycopg
        cls.sql = sql
        cls.schema = "ct_" + secrets.token_hex(4)

        cls.conn = psycopg.connect(DATABASE_URL)
        cls.conn.execute(
            sql.SQL("CREATE SCHEMA {}").format(sql.Identifier(cls.schema))
        )
        cls.conn.commit()

        # 迁移必须落在本用例的随机 schema（与 golang-migrate 的 search_path 纪律同源），
        # 否则 CREATE TABLE 进 public，后面的收窄查询就测了个空表。
        cls.conn.execute(
            sql.SQL("SET search_path TO {}").format(sql.Identifier(cls.schema))
        )
        # 真实执行本票迁移（不是复制一份 DDL）：保证契约测的是将被部署的同一文件。
        migration = next(MIGRATION_DIR.glob("000063_*.up.sql"))
        cls.conn.execute(migration.read_text(encoding="utf-8"))
        cls.conn.commit()

    @classmethod
    def tearDownClass(cls):
        cls.conn.execute(
            cls.sql.SQL("DROP SCHEMA {} CASCADE").format(
                cls.sql.Identifier(cls.schema)
            )
        )
        cls.conn.commit()
        cls.conn.close()

    def test_fixture_in_row_out(self):
        from collector.cli import run_batch

        count = run_batch(str(FIXTURE), DATABASE_URL, search_path=self.schema)
        self.assertEqual(count, 1)

        with self.psycopg.connect(
            DATABASE_URL, options=f"-c search_path={self.schema}"
        ) as conn:
            # 证明收窄真的落在随机 schema，而不是误写进 public。
            current = conn.execute("SELECT current_schema()").fetchone()[0]
            self.assertEqual(current, self.schema)

            table_count = conn.execute(
                "SELECT count(*) FROM information_schema.tables "
                "WHERE table_schema = current_schema() AND table_name = 'market_transactions'"
            ).fetchone()[0]
            self.assertEqual(table_count, 1)

            rows = conn.execute(
                "SELECT channel, source_url, raw_description FROM market_transactions"
            ).fetchall()

        self.assertEqual(len(rows), 1)
        channel, source_url, raw = rows[0]
        self.assertEqual(channel, "rmfysszc")
        self.assertEqual(source_url, EXPECTED_SOURCE_URL)
        self.assertEqual(raw["title"], EXPECTED_TITLE)
        self.assertEqual(raw["publish_date"], "2026-11-18")
        self.assertIn("起拍价 42000 元", raw["body"])


if __name__ == "__main__":
    unittest.main()
