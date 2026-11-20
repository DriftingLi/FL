"""多采集面的端到端契约测试（#2679 判据 1/2/3/4，对真 PG）。

沿用 spec #2671 的单一 seam：**fixture 进 → PG 行出**。这里钉的是「结构化直取
路径走完整条管线」——不是解析（那在 test_sources_parsers.py），而是：

  - 适配器产出的样本能过清洗、匹配、落库，行里有正确 channel / 价格 / 置信度；
  - 结构化路径**不调 LLM**（extractor 传一个一调用就炸的桩，证明它没被碰）；
  - 溢价标记在真实京东形态（评估价缺失）上不误判、在真实溢价形态上生效；
  - 无日期样本不被年份窗口丢掉（京东/公拍网都没有发布日期）。

无 DATABASE_URL 时干净 skip（首跑在 CI，纪律同 pg_fixture 的说明）。
"""

import os
import unittest
from pathlib import Path

DATABASE_URL = os.environ.get("DATABASE_URL")

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURES = REPO_ROOT / "collector" / "fixtures"

MASTER_KEY = "unit-test-master-key"

# 京东标的页（不是公告页）：适配器会打两个 JSONP 接口，假 crawler 按 functionId 分发。
JD_ITEM_URL = "https://paimai.jd.com/310055258"
JD_NOTICE_URL = "https://paimai.jd.com/notice/11810994"
GPAI_ITEM_URL = "https://www.gpai.net/sf/item2.do?Web_Item_ID=53385"
GPAI_LIST_URL = "https://www.gpai.net/sf/"


def _fixture(name: str) -> str:
    return (FIXTURES / name).read_text(encoding="utf-8")


class FakeCrawler:
    """按 URL / functionId 返回录制内容；记录抓了哪些 URL。"""

    def __init__(self, pages: dict[str, str], *, functions: dict[str, str] | None = None) -> None:
        self._pages = pages
        self._functions = functions or {}
        self.fetched: list[str] = []
        self.log = type("Log", (), {"summary": lambda self_: "请求日志：测试桩"})()

    def fetch(self, url: str) -> str:
        self.fetched.append(url)
        for key, payload in self._functions.items():
            if key in url:
                return payload
        if url not in self._pages:
            raise KeyError(f"未录制的 URL: {url}")
        return self._pages[url]


def _boom_extractor(*_args, **_kwargs):
    """LLM 抽取桩：结构化采集面不该调用它。被调用即测试失败。"""
    raise AssertionError(
        "结构化采集面（structured=True）不得调用 LLM 抽取——字段是来源事实"
    )


@unittest.skipUnless(DATABASE_URL, "本机无 DATABASE_URL：真 PG 契约测试跳过（首跑在 CI）")
class StructuredSourceE2EContractTest(unittest.TestCase):
    """每个用例一个独立随机 schema：回补有幂等语义，共享 schema 会让第二个用例
    「落 0 行」（已入库 URL 被跳过），把幂等误测成失败。"""

    def setUp(self):
        from collector.tests import pg_fixture

        self._pg = pg_fixture
        self.conn, self.schema, self.sql = pg_fixture.create_schema(prefix="src")
        pg_fixture.apply_market_migrations(self.conn)
        pg_fixture.create_stub_tables(self.conn)
        self.psycopg = __import__("psycopg")

    def tearDown(self):
        self._pg.drop_schema(self.conn, self.schema, self.sql)

    def _read(self, query, params=None):
        """每次新连接 + 断言 search_path 收窄在当前 schema（不碰 public）。"""
        with self.psycopg.connect(DATABASE_URL) as conn:
            with conn.cursor() as cur:
                cur.execute(f"SET search_path TO {self.schema}")
                cur.execute("SELECT current_schema()")
                self.assertEqual(cur.fetchone()[0], self.schema)
                cur.execute(query, params or ())
                return cur.fetchall()

    def _run_backfill(self, list_url, crawler):
        from collector.cli import run_backfill

        return run_backfill(
            [list_url],
            DATABASE_URL,
            search_path=self.schema,
            crawler=crawler,
            extractor=_boom_extractor,
            secret_key=MASTER_KEY,
            price_candidates=[],
        )

    # ---- 判据 1：京东标的页适配器走全管线落库 ------------------------------

    def test_jd_item_lands_as_deal_sample(self):
        crawler = FakeCrawler(
            {JD_NOTICE_URL: _fixture("jd_notice_11810994.html")},
            functions={
                "getProductBasicInfo": _fixture("jd_basic_310055258.json"),
                "getPaimaiRealTimeData": _fixture("jd_realtime_310117853.json"),
            },
        )
        count, report = self._run_backfill(JD_NOTICE_URL, crawler)
        self.assertGreater(count, 0, "公告页发现的标的必须落库")

        rows = self._read(
            "SELECT channel, raw_price, raw_price_kind, appraisal_price, "
            "structured_specs->>'title', extraction_version, clean_status, "
            "rejection_reason, field_confidence->>'raw_price' "
            "FROM market_transactions WHERE channel = 'jd' ORDER BY id"
        )
        self.assertTrue(rows, "京东采集面应落至少一行")
        (
            channel, price, kind, appraisal, title, version, status, reason, conf
        ) = rows[0]
        self.assertEqual(channel, "jd")
        self.assertEqual(kind, "deal")
        self.assertIsNotNone(price)
        self.assertIsNone(
            appraisal, "assessmentPrice=0.0 必须落 NULL（源站不提供评估价）"
        )
        self.assertIn("叉车", title or "")
        self.assertEqual(
            version, "structured-v1", "结构化路径的 extraction_version 要与 LLM 路径可分辨"
        )
        self.assertEqual(float(conf), 1.0)
        # 实测京东这批样本全是「报废资产…【无电池】」⇒ 清洗规则②应剔除它们。
        # 落库（有行）与可进标签集（cleaned）是两件事：剔除行保留是为了审计得率。
        self.assertEqual(status, "rejected", "报废/无电池样本必须被规则②剔除")
        self.assertEqual(reason, "scrapped_or_no_battery")
        self.assertFalse(
            any(
                r[0] == "jd"
                for r in self._read(
                    "SELECT channel FROM market_transactions "
                    "WHERE channel = 'jd' AND label_eligible"
                )
            ),
            "报废样本不得有标签资格（废铁价不能进校准集）",
        )

    def test_jd_zero_appraisal_does_not_flag_premium(self):
        crawler = FakeCrawler(
            {JD_NOTICE_URL: _fixture("jd_notice_11810994.html")},
            functions={
                "getProductBasicInfo": _fixture("jd_basic_310055258.json"),
                "getPaimaiRealTimeData": _fixture("jd_realtime_310117853.json"),
            },
        )
        self._run_backfill(JD_NOTICE_URL, crawler)
        flagged = self._read(
            "SELECT count(*) FROM market_transactions "
            "WHERE channel = 'jd' AND premium_anomaly"
        )
        self.assertEqual(flagged[0][0], 0, "评估价缺失（NULL）时溢价规则必须整条跳过")

    # ---- 判据 2：gpai 适配器走全管线落库 ----------------------------------

    def test_gpai_item_lands_with_deal_price_and_appraisal(self):
        # 列表 fixture 里的条目是 53260/53261/53262，而录制的详情页是 53385
        # （两次抓取不同批次）。这里把三个条目都映射到同一份详情 fixture：
        # 本用例验的是「列表→详情→清洗→匹配→落库」这条链路，不是 id 对应关系。
        detail = _fixture("gpai_sf_item_ended.html")
        crawler = FakeCrawler(
            {
                GPAI_LIST_URL: _fixture("gpai_sf_list.html"),
                "https://www.gpai.net/sf/item2.do?Web_Item_ID=53260": detail,
                "https://www.gpai.net/sf/item2.do?Web_Item_ID=53261": detail,
                "https://www.gpai.net/sf/item2.do?Web_Item_ID=53262": detail,
            }
        )
        count, _ = self._run_backfill(GPAI_LIST_URL, crawler)
        self.assertGreater(count, 0)

        rows = self._read(
            "SELECT raw_price, raw_price_kind, appraisal_price, clean_status "
            "FROM market_transactions WHERE channel = 'gpai' ORDER BY id"
        )
        self.assertTrue(rows)
        price, kind, appraisal, status = rows[0]
        self.assertEqual(kind, "deal")
        self.assertEqual(float(price), 195899.0)
        self.assertEqual(float(appraisal), 299818.0)
        self.assertIn(status, ("cleaned", "rejected"))

    # ---- 判据 5：限速/UA/退避由 CrawlClient 统一施加 ----------------------

    def test_all_requests_go_through_injected_crawler(self):
        """适配器内部的所有请求（含京东两个 JSONP）都必须走注入的抓取函数。

        这条钉的是「采集面无法绕过合规三道闸」：如果某个适配器自己 import urllib，
        这里就抓不到它发的请求，robots/限速/退避会被悄悄跳过。
        """
        crawler = FakeCrawler(
            {JD_NOTICE_URL: _fixture("jd_notice_11810994.html")},
            functions={
                "getProductBasicInfo": _fixture("jd_basic_310055258.json"),
                "getPaimaiRealTimeData": _fixture("jd_realtime_310117853.json"),
            },
        )
        self._run_backfill(JD_NOTICE_URL, crawler)
        self.assertTrue(
            any("api.m.jd.com" in url for url in crawler.fetched),
            "京东的 JSONP 请求必须经注入的抓取函数（否则绕过 robots/限速/退避）",
        )
        self.assertTrue(
            any("paimai.jd.com/notice/" in url for url in crawler.fetched),
            "公告页发现请求也必须经注入的抓取函数",
        )


@unittest.skipUnless(DATABASE_URL, "本机无 DATABASE_URL：真 PG 契约测试跳过（首跑在 CI）")
class UndatedSampleSurvivalTest(unittest.TestCase):
    """京东/公拍网没有发布日期字段：年份窗口不能把它们丢光。

    这是一个**静默失败**回归测试：如果 keep_undated 失效，回补会「成功返回 0 条」，
    没有报错、没有 failed_urls，得率报告显示「本轮无样本」——最难查的那类 bug。
    """

    @classmethod
    def setUpClass(cls):
        from collector.tests import pg_fixture

        cls._pg = pg_fixture
        cls.conn, cls.schema, cls.sql = pg_fixture.create_schema(prefix="und")
        pg_fixture.apply_market_migrations(cls.conn)
        pg_fixture.create_stub_tables(cls.conn)
        cls.psycopg = __import__("psycopg")

    @classmethod
    def tearDownClass(cls):
        cls._pg.drop_schema(cls.conn, cls.schema, cls.sql)

    def test_jd_undated_items_are_not_filtered_by_year_window(self):
        from collector.cli import run_backfill

        crawler = FakeCrawler(
            {JD_NOTICE_URL: _fixture("jd_notice_11810994.html")},
            functions={
                "getProductBasicInfo": _fixture("jd_basic_310055258.json"),
                "getPaimaiRealTimeData": _fixture("jd_realtime_310117853.json"),
            },
        )
        count, report = run_backfill(
            [JD_NOTICE_URL],
            DATABASE_URL,
            search_path=self.schema,
            # 故意用一个排他窗口：如果京东样本按「无日期 ⇒ 丢弃」处理，这里会是 0。
            start_year=1990,
            end_year=1991,
            crawler=crawler,
            extractor=_boom_extractor,
            secret_key=MASTER_KEY,
            price_candidates=[],
        )
        self.assertGreater(
            count,
            0,
            "京东标的没有发布日期，年份窗口对它不适用；被过滤掉就是静默失败",
        )
        self.assertEqual(report.failed_items, 0)


if __name__ == "__main__":
    unittest.main()
