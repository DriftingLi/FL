"""collector batch 端到端契约测试（spec #2671 指定的唯一 seam：fixture 进 → 行出）。

#2673 扩展：在真 PG 上钉住 LLM 抽取接线——
  - collector 从同库 ai_configs / ai_feature_bindings 直读配置并解密 api_key；
  - 抽取结果（结构化规格 / 逐字段置信度 / 抽取版本）写回同一 raw 行；
  - 低置信样本进 market_sample_review_queue；
  - 价格抽取失败样本 label_eligible=false 且进队列；
  - LLM 用录制 fixture 打桩，确定性，不测 prompt 质量。

纪律（与 backend/internal/testutil/pg.go 同源）：
  - 本机无 DATABASE_URL ⇒ 整个类干净 skip，首跑在 CI（collector-test job 提供真 PG）；
  - 随机 schema（ct_<hex>）隔离，迁移只落在本 schema；
  - 查询按 current_schema() 收窄，并显式断言 current_schema() == 本用例 schema；
  - tearDownClass DROP SCHEMA CASCADE 清理。

psycopg / cryptography 在 skip 判定之后才 import：本机既无 DATABASE_URL 也无驱动时不会因缺依赖报错。
"""

import json
import os
import secrets
import unittest
from pathlib import Path

DATABASE_URL = os.environ.get("DATABASE_URL")

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURES = REPO_ROOT / "collector" / "fixtures"
MIGRATION_DIR = REPO_ROOT / "backend" / "migrations"

EXPECTED_SOURCE_URL = "https://www.rmfysszc.com/auction/notice/2026/11001.html"
EXPECTED_TITLE = "杭叉 CPCD30 叉车公开拍卖公告"

FEATURE_KEY = "market_sample_extract"
# 与 test_crypto_interop.py 同一向量：Go security.EncryptSecret 真实产出。
ENCRYPTED_KEY = "enc:v1:ULpTLw09HcsKToqJbwY8777i33KFU0T7i1Rh1r3O82ub5SKMjvT9ODrltwB5YIs="
MASTER_KEY = "unit-test-master-key"


def _stub_from_file(name):
    """返回一个录制好的提取器 stub：只读 fixture，不打网络、不测 prompt 质量。"""
    payload = (FIXTURES / name).read_text(encoding="utf-8")

    def extractor(messages, config):
        # 顺带证明 collector 真的读到了配置（而不是绕过配置直接调 stub）。
        assert config.model == "stub-model", config
        assert config.api_key == "sk-test-vector-2673", config
        return payload

    return extractor


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
        cls.conn.execute(sql.SQL("CREATE SCHEMA {}").format(sql.Identifier(cls.schema)))
        cls.conn.commit()

        cls.conn.execute(sql.SQL("SET search_path TO {}").format(sql.Identifier(cls.schema)))
        # 执行真实迁移文件（不是复制 DDL）：000063 建样本表，000064 加标记列 + 队列，
        # 000065 加清洗版本列（#2674），000066 加匹配版本列（#2675 收口）。
        for pattern in (
            "000063_*.up.sql", "000064_*.up.sql", "000065_*.up.sql", "000066_*.up.sql",
        ):
            migration = next(MIGRATION_DIR.glob(pattern))
            cls.conn.execute(migration.read_text(encoding="utf-8"))
        cls.conn.commit()

        # 最小 original_prices 形状（镜像 000001 baseline 被读取的 7 字段 + 原价），
        # 让「从 PG 直读候选原价 + 分层匹配」在本 schema 内可断言。
        cls.conn.execute(
            """
            CREATE TABLE original_prices (
                id BIGSERIAL PRIMARY KEY,
                brand VARCHAR(50) NOT NULL,
                vehicle_type VARCHAR(50) NOT NULL,
                series VARCHAR(50) NOT NULL,
                tonnage NUMERIC(5,2) NOT NULL,
                config_type VARCHAR(50) NOT NULL,
                mast_type VARCHAR(50) NOT NULL,
                mast_height_mm INTEGER NOT NULL,
                original_price NUMERIC(12,2) NOT NULL
            )
            """
        )
        cls.conn.commit()

        # 最小 ai_configs / ai_feature_bindings 形状（镜像 000001 baseline 被读取的列），
        # 让「直读配置」在本 schema 内可断言，不必把整条 baseline 迁移灌进来。
        cls.conn.execute(
            """
            CREATE TABLE ai_configs (
                id BIGSERIAL PRIMARY KEY,
                name TEXT UNIQUE NOT NULL,
                api_key TEXT NOT NULL,
                base_url TEXT NOT NULL DEFAULT '',
                model TEXT NOT NULL DEFAULT '',
                is_active BOOLEAN NOT NULL DEFAULT TRUE
            )
            """
        )
        cls.conn.execute(
            """
            CREATE TABLE ai_feature_bindings (
                id BIGSERIAL PRIMARY KEY,
                feature_key VARCHAR(50) NOT NULL,
                config_id BIGINT NOT NULL REFERENCES ai_configs(id) ON DELETE CASCADE
            )
            """
        )
        cls.conn.execute(
            "INSERT INTO ai_configs (name, api_key, base_url, model) VALUES (%s, %s, %s, %s)",
            ("collector-stub", ENCRYPTED_KEY, "http://stub.invalid/v1", "stub-model"),
        )
        cls.conn.execute(
            "INSERT INTO ai_feature_bindings (feature_key, config_id) "
            "SELECT %s, id FROM ai_configs WHERE name = %s",
            (FEATURE_KEY, "collector-stub"),
        )
        cls.conn.commit()

    @classmethod
    def tearDownClass(cls):
        cls.conn.execute(
            cls.sql.SQL("DROP SCHEMA {} CASCADE").format(cls.sql.Identifier(cls.schema))
        )
        cls.conn.commit()
        cls.conn.close()

    def _read(self, query, params=None):
        with self.psycopg.connect(
            DATABASE_URL, options=f"-c search_path={self.schema}"
        ) as conn:
            current = conn.execute("SELECT current_schema()").fetchone()[0]
            self.assertEqual(current, self.schema)
            return conn.execute(query, params or ()).fetchall()

    def test_config_read_from_pg_and_decrypted(self):
        from collector.extract.config import resolve_config

        with self.psycopg.connect(
            DATABASE_URL, options=f"-c search_path={self.schema}"
        ) as conn:
            cfg = resolve_config(conn, FEATURE_KEY, MASTER_KEY)
        self.assertEqual(cfg.model, "stub-model")
        self.assertEqual(cfg.base_url, "http://stub.invalid/v1")
        self.assertEqual(cfg.api_key, "sk-test-vector-2673")

    def test_fixture_in_extracted_row_out(self):
        from collector.cli import run_batch

        count = run_batch(
            str(FIXTURES / "rmfysszc_announcement.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=_stub_from_file("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY,
        )
        self.assertEqual(count, 1)

        table_count = self._read(
            "SELECT count(*) FROM information_schema.tables "
            "WHERE table_schema = current_schema() AND table_name = 'market_transactions'"
        )[0][0]
        self.assertEqual(table_count, 1)

        rows = self._read(
            "SELECT channel, source_url, raw_description, structured_specs, "
            "field_confidence, extraction_version, raw_price, label_eligible "
            "FROM market_transactions WHERE source_url = %s",
            (EXPECTED_SOURCE_URL,),
        )
        self.assertEqual(len(rows), 1)
        (channel, source_url, raw, specs, confidence, version, price, eligible) = rows[0]
        self.assertEqual(channel, "rmfysszc")
        self.assertEqual(source_url, EXPECTED_SOURCE_URL)
        # 原文留底（title/publish_date/body）不被抽取覆盖——整块断言而非只钉标题。
        self.assertEqual(raw["title"], EXPECTED_TITLE)
        self.assertEqual(raw["publish_date"], "2026-11-18")
        self.assertIn("起拍价 42000 元", raw["body"])
        self.assertEqual(specs["brand"], "杭叉")
        self.assertEqual(specs["model"], "CPCD30")
        self.assertEqual(confidence["brand"], 0.97)
        self.assertTrue(version.startswith("v"))
        self.assertEqual(float(price), 42000.0)
        self.assertTrue(eligible)

        # 低置信字段 series=0.62 → 抽检队列有一行，指向样本 id。
        queue = self._read(
            "SELECT q.reason, q.detail FROM market_sample_review_queue q "
            "JOIN market_transactions m ON m.id = q.sample_id "
            "WHERE m.source_url = %s",
            (EXPECTED_SOURCE_URL,),
        )
        self.assertEqual(len(queue), 1)
        self.assertEqual(queue[0][0], "low_confidence")
        self.assertIn("series", queue[0][1])

    def test_price_extraction_failure_has_no_label_eligibility(self):
        from collector.cli import run_batch

        failed_url = "https://www.rmfysszc.com/auction/notice/2026/11002.html"
        fixture = FIXTURES / "rmfysszc_announcement_price_failed.html"
        count = run_batch(
            str(fixture),
            DATABASE_URL,
            search_path=self.schema,
            extractor=_stub_from_file("rmfysszc_extraction_stub_price_failed.json"),
            secret_key=MASTER_KEY,
        )
        self.assertEqual(count, 1)

        rows = self._read(
            "SELECT source_url, raw_price, label_eligible FROM market_transactions "
            "WHERE source_url = %s",
            (failed_url,),
        )
        self.assertEqual(len(rows), 1)
        self.assertIsNone(rows[0][1])
        self.assertFalse(rows[0][2])

        queue = self._read(
            "SELECT q.reason FROM market_sample_review_queue q "
            "JOIN market_transactions m ON m.id = q.sample_id "
            "WHERE m.source_url = %s",
            (failed_url,),
        )
        self.assertIn("price_extraction_failed", [r[0] for r in queue])

    # ---- #2674 清洗阶段：fixture 进 → 清洗状态 / 剔除原因 / 溢价标记 / 身份剥离 行出 ----

    def _stub_named_case(self, case_name):
        """从 clean_noise_cases.json 取某 case 的 extraction 作为录制抽取输出。"""
        cases = json.loads(
            (FIXTURES / "clean_noise_cases.json").read_text(encoding="utf-8")
        )
        payload = json.dumps(cases[case_name]["extraction"], ensure_ascii=False)
        return _stub_from_payload(payload)

    def test_clean_premium_is_marked_not_rejected(self):
        from collector.cli import run_batch

        url = "https://www.rmfysszc.com/auction/notice/2026/11003.html"
        count = run_batch(
            str(FIXTURES / "clean_premium.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=self._stub_named_case("premium"),
            secret_key=MASTER_KEY,
        )
        self.assertEqual(count, 1)
        rows = self._read(
            "SELECT clean_status, premium_anomaly, rejection_reason, label_eligible, clean_version "
            "FROM market_transactions WHERE source_url = %s",
            (url,),
        )
        self.assertEqual(len(rows), 1)
        status, premium, reason, eligible, clean_version = rows[0]
        self.assertEqual(status, "cleaned")
        self.assertTrue(premium)
        self.assertIsNone(reason)
        self.assertTrue(eligible)
        # 清洗版本必须落库，才可回答「这行是哪版规则判的」。
        self.assertEqual(clean_version, "v1")
        queue = self._read(
            "SELECT q.reason FROM market_sample_review_queue q "
            "JOIN market_transactions m ON m.id = q.sample_id WHERE m.source_url = %s",
            (url,),
        )
        self.assertIn("premium_anomaly", [r[0] for r in queue])

    def test_clean_bulk_lot_is_rejected_with_reason(self):
        from collector.cli import run_batch

        url = "https://www.rmfysszc.com/auction/notice/2026/11004.html"
        count = run_batch(
            str(FIXTURES / "clean_bulk_lot.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=self._stub_named_case("bulk_lot"),
            secret_key=MASTER_KEY,
        )
        self.assertEqual(count, 1)
        rows = self._read(
            "SELECT clean_status, rejection_reason, label_eligible "
            "FROM market_transactions WHERE source_url = %s",
            (url,),
        )
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0][0], "rejected")
        self.assertIn("bulk_lot_no_per_unit", rows[0][1])
        self.assertFalse(rows[0][2])

    def test_clean_identity_stripped_from_stored_row(self):
        from collector.cli import run_batch

        url = "https://www.rmfysszc.com/auction/notice/2026/11005.html"
        count = run_batch(
            str(FIXTURES / "clean_identity.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=self._stub_named_case("identity"),
            secret_key=MASTER_KEY,
        )
        self.assertEqual(count, 1)
        rows = self._read(
            "SELECT clean_status, structured_specs, raw_description "
            "FROM market_transactions WHERE source_url = %s",
            (url,),
        )
        self.assertEqual(len(rows), 1)
        status, specs, raw = rows[0]
        self.assertEqual(status, "cleaned")
        for key in ("buyer_name", "bidder_no", "id_no"):
            self.assertNotIn(key, specs)
            self.assertNotIn(key, raw)
        # PIPL：连原文留底正文里的行内身份值也不得落库。
        body = raw["body"]
        self.assertNotIn("张三", body)
        self.assertNotIn("A1234", body)
        self.assertIn("买受人", body)  # 非身份原文保留
        self.assertIn("42000 元竞得", body)

    def test_reclean_recomputes_without_refetch(self):
        """重跑清洗：直接 UPDATE 一行成 bulk_lot 特征，reclean 后重新判为 rejected。"""
        from collector.cli import run_reclean

        url = "https://www.rmfysszc.com/auction/notice/2026/11001.html"
        # 把已清洗的样本结构字段改成批量包价特征，用 reclean 重算（不重爬公告）。
        with self.psycopg.connect(
            DATABASE_URL, options=f"-c search_path={self.schema}"
        ) as conn:
            conn.execute(
                "UPDATE market_transactions SET structured_specs = %s "
                "WHERE source_url = %s",
                (json.dumps({"brand": "合力", "bulk_lot": True, "lot_count": 3}), url),
            )
            conn.commit()
        count = run_reclean(DATABASE_URL, search_path=self.schema)
        self.assertGreaterEqual(count, 1)
        rows = self._read(
            "SELECT clean_status, rejection_reason FROM market_transactions "
            "WHERE source_url = %s",
            (url,),
        )
        self.assertEqual(rows[0][0], "rejected")
        self.assertIn("bulk_lot_no_per_unit", rows[0][1])

    # ---- #2675 匹配阶段：分层 / 中位数 / 残值率 / 未匹配留空 ----

    def test_match_layer_median_residual_rate_written_back(self):
        """fixture（杭叉 CPCD30/series CPCD/3t，成交价 42000）→ 4 字段层命中，
        播 3 条原价候选 40000/50000/60000 → 中位数 50000、残值率 0.84、区间入表、版本落库。
        """
        from collector.cli import run_batch

        with self.psycopg.connect(
            DATABASE_URL, options=f"-c search_path={self.schema}"
        ) as conn:
            with conn.cursor() as cur:
                cur.executemany(
                    "INSERT INTO original_prices "
                    "(brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) "
                    "VALUES (%s, %s, %s, %s, %s, %s, %s, %s)",
                    [
                        ("杭叉", "CPCD30", "CPCD", 3.0, "A", "二节", 3000, 40000),
                        ("杭叉", "CPCD30", "CPCD", 3.0, "B", "二节", 3000, 50000),
                        ("杭叉", "CPCD30", "CPCD", 3.0, "C", "三节", 4500, 60000),
                    ],
                )
            conn.commit()

        count = run_batch(
            str(FIXTURES / "rmfysszc_announcement.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=_stub_from_file("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY,
        )
        self.assertEqual(count, 1)
        rows = self._read(
            "SELECT match_level, matched_original_price, matched_original_range, "
            "residual_rate_label, match_version, label_eligible "
            "FROM market_transactions WHERE source_url = %s",
            (EXPECTED_SOURCE_URL,),
        )
        # 该 URL 在前面的用例也插过行；取刚插入的匹配结果不为真空的那条。
        matched = [r for r in rows if r[0] is not None]
        self.assertGreaterEqual(len(matched), 1)
        level, original, rng, residual, version, eligible = matched[0]
        self.assertEqual(level, "brand_vehicle_series_tonnage")
        self.assertEqual(float(original), 50000.0)  # 中位数，不是最高价 60000
        self.assertEqual(rng["min"], 40000.0)
        self.assertEqual(rng["max"], 60000.0)
        self.assertEqual(rng["count"], 3)
        self.assertAlmostEqual(float(residual), 42000 / 50000, places=6)
        self.assertEqual(version, "v1")  # 精确钉版本，不只 startswith('v')
        self.assertTrue(eligible)

    def test_unmatched_keeps_label_eligible_with_null_label(self):
        """无候选原价 → 不写标签、不进校准集，但 label_eligible 保持价格抽取成功语义。"""
        from collector.cli import run_batch

        count = run_batch(
            str(FIXTURES / "rmfysszc_announcement.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=_stub_from_file("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY,
            price_candidates=[],  # 强制无匹配，且本用例不依赖前面播的价格表
        )
        self.assertEqual(count, 1)
        rows = self._read(
            "SELECT match_level, matched_original_price, residual_rate_label, label_eligible "
            "FROM market_transactions WHERE source_url = %s ORDER BY id DESC LIMIT 1",
            (EXPECTED_SOURCE_URL,),
        )
        self.assertEqual(len(rows), 1)
        self.assertIsNone(rows[0][0])
        self.assertIsNone(rows[0][1])
        self.assertIsNone(rows[0][2])
        self.assertTrue(rows[0][3])

    def _run_match_layer(self, url, extraction, candidates):
        """用 match_layer.html fixture + 指定抽取 payload 在真 PG 上跑一次 batch。"""
        from collector.cli import run_batch

        payload = json.dumps(extraction, ensure_ascii=False)
        count = run_batch(
            str(FIXTURES / "match_layer.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=_stub_from_payload(payload),
            secret_key=MASTER_KEY,
            price_candidates=candidates,
        )
        self.assertEqual(count, 1)
        return self._read(
            "SELECT match_level, matched_original_price, residual_rate_label, match_version "
            "FROM market_transactions WHERE source_url = %s ORDER BY id DESC LIMIT 1",
            (url,),
        )[0]

    def test_exact7_layer_on_real_pg(self):
        """7 字段全齐 → exact7 层命中，版本精确为 v1（不是只 startswith('v')）。"""
        extraction = {
            "structured_specs": {
                "brand": "杭叉", "model": "CPCD30", "series": "CPCD", "tonnage": 3.0,
                "config_type": "标准", "mast_type": "二节", "mast_height_mm": 3000,
            },
            "field_confidence": {"brand": 0.9},
            "raw_price": 42000, "raw_price_kind": "deal",
        }
        candidates = [{
            "brand": "杭叉", "vehicle_type": "CPCD30", "series": "CPCD", "tonnage": 3.0,
            "config_type": "标准", "mast_type": "二节", "mast_height_mm": 3000,
            "original_price": 50000.0,
        }]
        url = "https://www.rmfysszc.com/auction/notice/2026/12001.html"
        level, original, residual, version = self._run_match_layer(url, extraction, candidates)
        self.assertEqual(level, "exact7")
        self.assertEqual(float(original), 50000.0)
        self.assertAlmostEqual(float(residual), 42000 / 50000, places=6)
        self.assertEqual(version, "v1")

    def test_brand_vehicle_tonnage_layer_on_real_pg(self):
        """只给 品牌+车型+吨位（缺 series/config/mast）→ 3 字段层命中。"""
        extraction = {
            "structured_specs": {"brand": "合力", "model": "CPCD50", "tonnage": 5.0},
            "field_confidence": {"brand": 0.9},
            "raw_price": 60000, "raw_price_kind": "deal",
        }
        candidates = [{
            "brand": "合力", "vehicle_type": "CPCD50", "series": "CPCD-X", "tonnage": 5.0,
            "config_type": "标准", "mast_type": "二节", "mast_height_mm": 3000,
            "original_price": 80000.0,
        }]
        url = "https://www.rmfysszc.com/auction/notice/2026/12001.html"
        level, original, residual, version = self._run_match_layer(url, extraction, candidates)
        self.assertEqual(level, "brand_vehicle_tonnage")
        self.assertEqual(float(original), 80000.0)
        self.assertAlmostEqual(float(residual), 60000 / 80000, places=6)

    def test_wrong_value_candidate_excluded_on_real_pg(self):
        """列齐全但规格值不同的原价不得命中——真表全 NOT NULL，此用例防「全表误匹配」。"""
        extraction = {
            "structured_specs": {
                "brand": "杭叉", "model": "CPCD30", "series": "CPCD", "tonnage": 3.0,
            },
            "field_confidence": {"brand": 0.9},
            "raw_price": 42000, "raw_price_kind": "deal",
        }
        candidates = [
            {"brand": "合力", "vehicle_type": "CPCD50", "series": "CPCD", "tonnage": 5.0,
             "config_type": "A", "mast_type": "二节", "mast_height_mm": 3000,
             "original_price": 1000000.0},
        ]
        url = "https://www.rmfysszc.com/auction/notice/2026/12001.html"
        level, original, residual, version = self._run_match_layer(url, extraction, candidates)
        self.assertIsNone(level)
        self.assertIsNone(original)
        self.assertIsNone(residual)

    def test_rematch_recovers_label_without_refetch(self):
        """补入原价后 rematch：未匹配行回收标签，抽取/清洗列不动。"""
        from collector.cli import run_batch, run_rematch

        url = "https://www.rmfysszc.com/auction/notice/2026/12002.html"
        # 先用空候选落一行未匹配样本（独立 URL，避免跨用例干扰）。
        run_batch(
            str(FIXTURES / "match_rematch.html"),
            DATABASE_URL,
            search_path=self.schema,
            extractor=_stub_from_file("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY,
            price_candidates=[],
        )
        before = self._read(
            "SELECT id, extraction_version, clean_version, match_level, residual_rate_label "
            "FROM market_transactions WHERE source_url = %s",
            (url,),
        )[0]
        self.assertIsNone(before[3])  # 当前未匹配

        # 只对该样本补入原价；rematch 读回全表但对其它行无害（候选只匹配本规格）。
        candidates = [{
            "brand": "杭叉", "vehicle_type": "CPCD30", "series": "CPCD", "tonnage": 3.0,
            "config_type": "标准", "mast_type": "二节", "mast_height_mm": 3000,
            "original_price": 50000.0,
        }]
        count = run_rematch(
            DATABASE_URL, search_path=self.schema, price_candidates=candidates
        )
        self.assertGreaterEqual(count, 1)
        after = self._read(
            "SELECT extraction_version, clean_version, match_level, residual_rate_label "
            "FROM market_transactions WHERE id = %s",
            (before[0],),
        )[0]
        self.assertEqual(after[0], before[1])  # 抽取版本不动
        self.assertEqual(after[1], before[2])  # 清洗版本不动
        self.assertEqual(after[2], "brand_vehicle_series_tonnage")
        self.assertAlmostEqual(float(after[3]), 42000 / 50000, places=6)


def _stub_from_payload(payload):
    """录制好的提取器 stub：返回固定 payload，同时证明配置被读取并解密。"""
    def extractor(messages, config):
        assert config.model == "stub-model", config
        assert config.api_key == "sk-test-vector-2673", config
        return payload
    return extractor


if __name__ == "__main__":
    unittest.main()
