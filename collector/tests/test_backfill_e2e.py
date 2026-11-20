"""历史回补的端到端契约测试（#2677 判据 1/3/5，对真 PG）。

沿用 spec #2671 的单一 seam 纪律：fixture 进 → 行出，对真 PG 断言。
回补比 batch 多两层（发现 + 编排），所以这里钉的是「编排层的行为」：
  - 判据 1：一条命令（run_backfill）从列表页 URL 走到落库，不需要中间手工步骤；
  - 判据 3：回补样本走全管线——落库行有 clean_status，且能带标签/剔除原因；
  - 判据 5：已入库公告幂等——同一 URL 重跑不重复落行。

抓取层用注入的假 crawler（`fetch(url) -> html`）：不碰网络、不碰 robots，
robots/限速/退避由 test_crawl_* 的纯判定单测覆盖。本文件不测那三件事。
"""

import json
import os
import unittest
from pathlib import Path

DATABASE_URL = os.environ.get("DATABASE_URL")

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURES = REPO_ROOT / "collector" / "fixtures"

# 与 pg_fixture 保持同一组常量（那里才是唯一真相，这里只为少写一次 import 前缀）。
MASTER_KEY = "unit-test-master-key"

LIST_PAGE = "https://www1.rmfysszc.gov.cn/Agency/pmgg.aspx?fid=100"
DETAIL_A = "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/14001.shtml"
DETAIL_B = "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/14002.shtml"

LIST_HTML = f"""
<html><body><table><tbody>
<tr><td><a href="{DETAIL_A}" title="杭叉 CPCD30 拍卖公告">杭叉 CPCD30 拍卖公告</a></td>
    <td>某法院</td><td><span class="n_c_r">2020.05.11</span></td></tr>
<tr><td><a href="{DETAIL_B}" title="合力 CPCD50 拍卖公告">合力 CPCD50 拍卖公告</a></td>
    <td>某法院</td><td><span class="n_c_r">2021.06.12</span></td></tr>
</tbody></table></body></html>
"""


def _detail_html(title: str, body: str) -> str:
    return f"""
<html><head><link rel="canonical" href="{DETAIL_A}"></head>
<body><div class="xmxx_main"><div id="Title"><h1>{title}</h1></div>
<div id="pmgg"><div class="notice-body"><p>{body}</p></div></div>
</div></body></html>
"""


class FakeCrawler:
    """注入的抓取层：按 URL 返回录好的 HTML，并记录抓了哪些 URL。"""

    def __init__(self, pages: dict[str, str]) -> None:
        self._pages = pages
        self.fetched: list[str] = []
        self.log = type("Log", (), {"summary": lambda self_: "请求日志：测试桩"})()

    def fetch(self, url: str) -> str:
        self.fetched.append(url)
        if url not in self._pages:
            raise KeyError(f"未录制的 URL: {url}")
        return self._pages[url]


@unittest.skipUnless(DATABASE_URL, "本机无 DATABASE_URL：真 PG 契约测试跳过（首跑在 CI）")
class BackfillE2EContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        from collector.tests import pg_fixture

        cls._pg = pg_fixture
        cls.conn, cls.schema, cls.sql = pg_fixture.create_schema(prefix="bf")
        pg_fixture.apply_market_migrations(cls.conn)
        pg_fixture.create_stub_tables(cls.conn)
        cls.psycopg = __import__("psycopg")

    @classmethod
    def tearDownClass(cls):
        cls._pg.drop_schema(cls.conn, cls.schema, cls.sql)

    def _read(self, query, params=None):
        with self.psycopg.connect(
            DATABASE_URL, options=f"-c search_path={self.schema}"
        ) as conn:
            current = conn.execute("SELECT current_schema()").fetchone()[0]
            self.assertEqual(current, self.schema)
            return conn.execute(query, params or ()).fetchall()

    def _clean_table(self):
        with self.psycopg.connect(
            DATABASE_URL, options=f"-c search_path={self.schema}"
        ) as conn:
            conn.execute("DELETE FROM market_sample_review_queue")
            conn.execute("DELETE FROM market_transactions")
            conn.commit()

    def _stub(self, payload_name: str):
        payload = (FIXTURES / payload_name).read_text(encoding="utf-8")

        def extractor(messages, config):
            assert config.model == "stub-model", config
            return payload

        return extractor

    def test_one_command_backfills_from_list_page_to_rows(self):
        """判据 1 + 3：列表页 URL 进 → 行出，走全管线（带清洗状态）。"""
        from collector.cli import run_backfill

        self._clean_table()
        crawler = FakeCrawler({
            LIST_PAGE: LIST_HTML,
            DETAIL_A: _detail_html("杭叉 CPCD30 拍卖公告", "起拍价 42000 元"),
            DETAIL_B: _detail_html("合力 CPCD50 拍卖公告", "起拍价 88000 元"),
        })
        stored, report = run_backfill(
            [LIST_PAGE],
            DATABASE_URL,
            search_path=self.schema,
            crawler=crawler,
            extractor=self._stub("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY,
            price_candidates=[],
        )
        self.assertEqual(stored, 2, "两条公告都应落库")
        self.assertIn(LIST_PAGE, crawler.fetched)
        self.assertIn(DETAIL_A, crawler.fetched)
        self.assertIn(DETAIL_B, crawler.fetched)

        rows = self._read(
            "SELECT source_url, clean_status, extraction_version FROM market_transactions "
            "ORDER BY source_url"
        )
        self.assertEqual(len(rows), 2)
        urls = [r[0] for r in rows]
        self.assertIn(DETAIL_A, urls)
        self.assertIn(DETAIL_B, urls)
        # 判据 3：回补样本走全管线 ⇒ 有清洗状态、有抽取版本。
        for _, status, version in rows:
            self.assertIn(status, ("raw", "cleaned", "rejected"))
            self.assertIsNotNone(version, "回补样本应经过抽取阶段")

        # 判据 4：得率报告产出，记录为事实。
        self.assertEqual(report.total, 2)
        self.assertIn("得率", report.render())

    def test_rerun_is_idempotent(self):
        """判据 5：已入库公告重跑不重复落行。"""
        from collector.cli import run_backfill

        self._clean_table()
        pages = {
            LIST_PAGE: LIST_HTML,
            DETAIL_A: _detail_html("杭叉 CPCD30 拍卖公告", "起拍价 42000 元"),
            DETAIL_B: _detail_html("合力 CPCD50 拍卖公告", "起拍价 88000 元"),
        }
        first_crawler = FakeCrawler(pages)
        first, _ = run_backfill(
            [LIST_PAGE], DATABASE_URL, search_path=self.schema,
            crawler=first_crawler,
            extractor=self._stub("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY, price_candidates=[],
        )
        self.assertEqual(first, 2)

        second_crawler = FakeCrawler(pages)
        second, report = run_backfill(
            [LIST_PAGE], DATABASE_URL, search_path=self.schema,
            crawler=second_crawler,
            extractor=self._stub("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY, price_candidates=[],
        )
        self.assertEqual(second, 0, "重跑不得重复落行")
        # 已入库的详情页根本不该再抓（省流量也是幂等的一部分）。
        self.assertNotIn(DETAIL_A, second_crawler.fetched)
        self.assertNotIn(DETAIL_B, second_crawler.fetched)

        total = self._read("SELECT count(*) FROM market_transactions")[0][0]
        self.assertEqual(total, 2, "重跑后总行数不变")
        self.assertEqual(report.total, 0, "本轮无新样本")

    def test_year_filter_excludes_out_of_window(self):
        """判据 1：回补窗口 2015–2026，窗口外公告不抓不落。"""
        from collector.cli import run_backfill

        self._clean_table()
        old_url = "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/14000.shtml"
        list_html = LIST_HTML.replace("2020.05.11", "2010.05.11")
        crawler = FakeCrawler({
            LIST_PAGE: list_html,
            DETAIL_B: _detail_html("合力 CPCD50 拍卖公告", "起拍价 88000 元"),
        })
        stored, _ = run_backfill(
            [LIST_PAGE], DATABASE_URL, search_path=self.schema,
            crawler=crawler,
            extractor=self._stub("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY, price_candidates=[],
        )
        self.assertEqual(stored, 1, "只有窗口内的那条应回补")
        self.assertNotIn(old_url, crawler.fetched)

    def test_rejected_sample_lands_with_reason_for_yield_report(self):
        """判据 3 + 4：被剔除样本也落库并保留原因，得率报告能统计到。"""
        from collector.cli import run_backfill

        self._clean_table()
        # 批量包价 fixture：清洗规则①会剔除，reason=bulk_lot_no_per_unit。
        bulk_html = (FIXTURES / "clean_bulk_lot.html").read_text(encoding="utf-8")
        bulk_payload = json.dumps({
            "structured_specs": {"brand": "合力", "lot_count": 3, "bulk_lot": True},
            "field_confidence": {"brand": 0.9, "raw_price": 0.9},
            "raw_price": 90000,
            "raw_price_kind": "listing",
            "appraisal_price": 90000,
        })
        payloads = {
            "bulk": bulk_payload,
            "normal": (FIXTURES / "rmfysszc_extraction_stub.json").read_text(encoding="utf-8"),
        }

        def extractor(messages, config):
            # 按公告正文区分该用哪份录制 payload（测试桩，不测 prompt 质量）。
            assert config.model == "stub-model", config
            text = " ".join(str(m.get("content", "")) for m in messages)
            return payloads["bulk"] if "打包" in text else payloads["normal"]

        crawler = FakeCrawler({
            LIST_PAGE: LIST_HTML,
            DETAIL_A: bulk_html,
            DETAIL_B: _detail_html("合力 CPCD50 拍卖公告", "起拍价 88000 元"),
        })
        _, report = run_backfill(
            [LIST_PAGE], DATABASE_URL, search_path=self.schema,
            crawler=crawler,
            extractor=extractor,
            secret_key=MASTER_KEY, price_candidates=[],
        )
        rejected = self._read(
            "SELECT rejection_reason FROM market_transactions "
            "WHERE clean_status = 'rejected'"
        )
        self.assertGreaterEqual(len(rejected), 1, "批量包价样本应被剔除并落库")
        self.assertIn("bulk_lot_no_per_unit", [r[0] for r in rejected])
        self.assertGreaterEqual(report.by_reason.get("bulk_lot_no_per_unit", 0), 1)

    def test_one_item_failure_does_not_rollback_prior_items(self):
        """每条一个独立事务 + 毒丸隔离——第 N 条抓取失败既不回滚前面已落的行，
        也不再让整轮抛错（常驻 daemon 靠这个才不会永久卡在队头）。"""
        from collector.cli import run_backfill

        self._clean_table()
        # 第 2 条详情页抓不到（未录制 ⇒ FakeCrawler 抛 KeyError）。
        crawler = FakeCrawler({
            LIST_PAGE: LIST_HTML,
            DETAIL_A: _detail_html("杭叉 CPCD30 拍卖公告", "起拍价 42000 元"),
            # DETAIL_B 故意不录制。
        })
        stored, report = run_backfill(
            [LIST_PAGE], DATABASE_URL, search_path=self.schema,
            crawler=crawler,
            extractor=self._stub("rmfysszc_extraction_stub.json"),
            secret_key=MASTER_KEY, price_candidates=[],
        )
        # 第 1 条已落库，不因第 2 条失败而被回滚。
        total = self._read("SELECT count(*) FROM market_transactions")[0][0]
        self.assertEqual(total, 1, "单条失败不应回滚同轮已落库的行")
        self.assertEqual(stored, 1, "应如实返回落库行数")
        # 失败的那条如实进报告，且不阻塞整轮（不再抛 KeyError）。
        self.assertEqual(report.failed_items, 1, "失败的 URL 应记入得率报告")
        self.assertIn(DETAIL_B, report.failed_urls)

    def test_cross_url_dedup_sees_prior_url_samples(self):
        """审阅修复：清洗规则⑥的车商去重能看见同轮其它 URL 的样本（image hash）。"""
        from collector.cli import run_backfill

        self._clean_table()
        # 两条 URL 抽取出的样本 image_hashes 有交集 ⇒ 第二条应被规则⑥剔除。
        p_normal = (FIXTURES / "rmfysszc_extraction_stub.json").read_text(encoding="utf-8")
        payload_dup = json.dumps({
            "structured_specs": {
                "brand": "杭叉", "model": "CPCD30", "tonnage": 3.0,
                "image_hashes": ["aaa111", "bbb222"],
            },
            "field_confidence": {"brand": 0.95, "raw_price": 0.95},
            "raw_price": 42000,
            "raw_price_kind": "deal",
            "appraisal_price": 40000,
        })
        payload_first = json.dumps({
            "structured_specs": {
                "brand": "杭叉", "model": "CPCD30", "tonnage": 3.0,
                "image_hashes": ["aaa111", "other999"],
            },
            "field_confidence": {"brand": 0.95, "raw_price": 0.95},
            "raw_price": 42000,
            "raw_price_kind": "deal",
            "appraisal_price": 40000,
        })
        # 按消息体里能区分的标记分派 payload（测试桩）。
        def extractor(messages, config):
            assert config.model == "stub-model", config
            text = " ".join(str(m.get("content", "")) for m in messages)
            # 第二条正文带特定标记。
            return payload_dup if "图二" in text else payload_first

        crawler = FakeCrawler({
            LIST_PAGE: LIST_HTML,
            DETAIL_A: _detail_html("杭叉 CPCD30 公告", "起拍价 42000 元 标的甲"),
            DETAIL_B: _detail_html("杭叉 CPCD30 公告", "起拍价 42000 元 图二 标的乙"),
        })
        _, report = run_backfill(
            [LIST_PAGE], DATABASE_URL, search_path=self.schema,
            crawler=crawler,
            extractor=extractor,
            secret_key=MASTER_KEY, price_candidates=[],
        )
        dup_reasons = [r[0] for r in self._read(
            "SELECT rejection_reason FROM market_transactions "
            "WHERE rejection_reason LIKE %s",
            ("duplicate%",),
        )]
        self.assertIn("duplicate_image_hash", dup_reasons,
                      "跨 URL 的图哈希重复应被规则⑥剔除")


if __name__ == "__main__":
    unittest.main()
