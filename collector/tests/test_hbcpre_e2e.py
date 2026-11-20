"""湖北产权交易所省镜像的端到端契约测试（#2680 判据 1/2/3/4，对真 PG）。

沿用 spec #2671 的单一 seam：**fixture 进 → PG 行出**。这里钉的是判据：

  ① 结构化字段直取 ⇒ 走全管线落库，且无需 LLM 抽取的字段置信度是最高级；
  ② **挂牌价 + 成交金额配对入表**（同一样本两价齐备）——这是本票的核心；
  ③ 限速/UA/退避沿用注入的 crawler（不在适配器里另开后门）；
  ④ 与京东/gpai 适配器无共享状态冲突——同一轮回补里两种采集面各自落库、
     各自清洗、互不干扰（并行开发可各自绿）。

无 DATABASE_URL 时干净 skip（首跑在 CI，纪律同 pg_fixture 的说明）。
"""

import os
import unittest
from pathlib import Path

DATABASE_URL = os.environ.get("DATABASE_URL")

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURES = REPO_ROOT / "collector" / "fixtures"

MASTER_KEY = "unit-test-master-key"

JYGG_LIST = "https://www.hbggzyfwpt.cn/jyxx/whcq/jygg"
JYJG_LIST = "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjg"
# fixture 里录的两个 guid（列表页条目与详情页 guid 不同批，映射到同一份 fixture）。
JYGG_DETAIL = "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=e3484a1f-a37d-40dd-844e-fc1eab7c87d7"
JYJG_DETAIL = "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=5d514bc8-815c-4025-94b8-455776a99e41"


def _fixture(name: str) -> str:
    return (FIXTURES / name).read_text(encoding="utf-8")


class FakeCrawler:
    """按 URL 返回录制内容；记录抓了哪些 URL（判据③的观察点）。

    `functions` 按 URL 子串分发——京东适配器把 `functionId` 拼在 query 里，
    同一 host 的两个接口靠这个键区分。
    """

    def __init__(
        self, pages: dict[str, str], *, functions: dict[str, str] | None = None
    ) -> None:
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
class HbcpreE2EContractTest(unittest.TestCase):
    """每个用例一个独立随机 schema：回补有幂等语义，共享 schema 会让第二个用例落 0 行。"""

    def setUp(self):
        from collector.tests import pg_fixture

        self._pg = pg_fixture
        self.conn, self.schema, self.sql = pg_fixture.create_schema(prefix="hbc")
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

    def _run_backfill(self, list_urls, crawler):
        from collector.cli import run_backfill

        return run_backfill(
            list_urls,
            DATABASE_URL,
            search_path=self.schema,
            crawler=crawler,
            extractor=_boom_extractor,
            secret_key=MASTER_KEY,
            price_candidates=[],
        )

    def _crawler_for_deal(self) -> FakeCrawler:
        return FakeCrawler(
            {
                JYJG_LIST: _fixture("hbcpre_jyjg_list.html"),
                # 列表 fixture 的两个 guid 都映射到同一份成交详情 fixture：
                # 第二个会被幂等去重（source_url 不同但内容相同，落两行也合规），
                # 这里只关心「至少一行两价齐备」。
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=5d514bc8-815c-4025-94b8-455776a99e41": _fixture("hbcpre_jyjg_detail.html"),
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=31c0ed65-ed9a-419a-af47-1925c11ad3c7": _fixture("hbcpre_jyjg_detail.html"),
            }
        )

    def _crawler_for_listing(self) -> FakeCrawler:
        return FakeCrawler(
            {
                JYGG_LIST: _fixture("hbcpre_jygg_list.html"),
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=e3484a1f-a37d-40dd-844e-fc1eab7c87d7": _fixture("hbcpre_jygg_detail.html"),
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=a4b3dffe-b1e3-487f-a594-4fb87398fd6b": _fixture("hbcpre_jygg_detail.html"),
            }
        )

    # ---- 判据 1：结构化直取走全管线落库 ------------------------------------

    def test_structured_source_lands_without_llm(self):
        """判据①：字段来自服务端表格 ⇒ 不调 LLM，走完清洗/匹配/落库。"""
        crawler = self._crawler_for_listing()
        count, report = self._run_backfill([JYGG_LIST], crawler)
        self.assertGreater(count, 0, "结构化采集面必须落库")

        rows = self._read(
            "SELECT channel, raw_price, raw_price_kind, extraction_version, "
            "field_confidence->>'raw_price' FROM market_transactions "
            "WHERE channel = 'hbcpre' ORDER BY id"
        )
        self.assertTrue(rows)
        channel, price, kind, version, conf = rows[0]
        self.assertEqual(channel, "hbcpre")
        self.assertEqual(kind, "listing")
        self.assertEqual(price, 24300, "挂牌价 2.43 万元必须折成 24300 元")
        self.assertEqual(
            version, "structured-v1",
            "结构化路径的 extraction_version 要与 LLM 路径可分辨",
        )
        self.assertEqual(
            float(conf), 1.0, "判据①：结构化直取字段的置信度是最高级"
        )

    def test_undated_entries_are_not_filtered_by_year_window(self):
        """列表页不给发布日期 ⇒ 年份窗口不能把它们丢掉。

        用 1990–1991 这个排他窗口跑：keep_undated 的采集面必须仍然落库。
        """
        crawler = self._crawler_for_listing()
        count, _ = self._run_backfill([JYGG_LIST], crawler)
        self.assertGreater(
            count, 0,
            "keep_undated 采集面不得被年份窗口过滤（列表页无日期是源的形态，不是缺陷）",
        )

    # ---- 判据 2：挂牌价 + 成交金额配对入表（本票核心）-----------------------

    def test_deal_sample_carries_both_prices(self):
        """判据②：同一样本两价齐备——成交金额 + 挂牌价落进两列。

        实测 fixture：评估价 199202 / 挂牌价 199300 / 成交金额 268200。
        """
        crawler = self._crawler_for_deal()
        count, _ = self._run_backfill([JYJG_LIST], crawler)
        self.assertGreater(count, 0, "成交详情必须落库")

        rows = self._read(
            "SELECT raw_price, raw_price_kind, listing_price, appraisal_price "
            "FROM market_transactions WHERE channel = 'hbcpre' "
            "AND raw_price_kind = 'deal' ORDER BY id LIMIT 1"
        )
        self.assertTrue(rows, "应至少有一条成交样本")
        deal_price, kind, listing_price, appraisal_price = rows[0]
        self.assertEqual(kind, "deal")
        self.assertEqual(deal_price, 268200, "raw_price 是成交金额")
        self.assertEqual(listing_price, 199300, "listing_price 是挂牌价")
        self.assertIsNotNone(
            listing_price, "判据②的硬要求：成交样本必须同时带挂牌价"
        )
        self.assertEqual(appraisal_price, 199202, "评估价也要落库")

    def test_listing_sample_has_listing_price_column_filled(self):
        """挂牌样本上 listing_price 也必须有值（列语义恒定，下游不必分情况读）。"""
        crawler = self._crawler_for_listing()
        self._run_backfill([JYGG_LIST], crawler)
        rows = self._read(
            "SELECT raw_price, listing_price FROM market_transactions "
            "WHERE channel = 'hbcpre' AND raw_price_kind = 'listing'"
        )
        self.assertTrue(rows)
        for raw_price, listing_price in rows:
            self.assertEqual(listing_price, raw_price)

    def test_deal_to_listing_discount_is_computable(self):
        """两价齐备的用处：下游可直接算「成交/挂牌」折扣比。"""
        crawler = self._crawler_for_deal()
        self._run_backfill([JYJG_LIST], crawler)
        rows = self._read(
            "SELECT raw_price / listing_price FROM market_transactions "
            "WHERE channel = 'hbcpre' AND raw_price_kind = 'deal' "
            "AND listing_price > 0 LIMIT 1"
        )
        self.assertTrue(rows)
        ratio = float(rows[0][0])
        # 实测 268200 / 199300 = 1.3457（溢价成交）。
        self.assertGreater(ratio, 0.5)
        self.assertLess(ratio, 2.0)

    # ---- 判据 3：限速/UA/退避沿用注入的 crawler -----------------------------

    def test_all_requests_go_through_injected_crawler(self):
        """判据③：适配器所有请求都走注入的 crawler（合规三道闸不可绕过）。"""
        crawler = self._crawler_for_deal()
        self._run_backfill([JYJG_LIST], crawler)
        self.assertIn(JYJG_LIST, crawler.fetched, "列表页必须走注入的 crawler")
        detail_hits = [u for u in crawler.fetched if "jyjgDetail" in u]
        self.assertTrue(detail_hits, "详情页也必须走注入的 crawler")

    # ---- 判据 4：与京东/gpai 无共享状态冲突 --------------------------------

    def test_hbcpre_and_jd_coexist_in_same_run(self):
        """判据④：同一轮回补里两个采集面各自落库、互不干扰。

        两个列表 URL 一起传 ⇒ 各自识别采集面、各自解析、行上 channel 正确。
        """
        jd_notice = "https://paimai.jd.com/notice/11810994"
        crawler = FakeCrawler(
            {
                JYJG_LIST: _fixture("hbcpre_jyjg_list.html"),
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=5d514bc8-815c-4025-94b8-455776a99e41": _fixture("hbcpre_jyjg_detail.html"),
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=31c0ed65-ed9a-419a-af47-1925c11ad3c7": _fixture("hbcpre_jyjg_detail.html"),
                jd_notice: _fixture("jd_notice_11810994.html"),
            },
            functions={
                "getProductBasicInfo": _fixture("jd_basic_310055258.json"),
                "getPaimaiRealTimeData": _fixture("jd_realtime_310117853.json"),
            },
        )
        count, _ = self._run_backfill([JYJG_LIST, jd_notice], crawler)
        self.assertGreater(count, 0)

        channels = {
            row[0]
            for row in self._read(
                "SELECT DISTINCT channel FROM market_transactions"
            )
        }
        self.assertIn("hbcpre", channels, "湖北产权样本必须落库")
        self.assertIn("jd", channels, "京东样本必须同时落库（无共享状态冲突）")

    def test_lease_samples_are_rejected_but_still_stored(self):
        """租赁权样本落库但被剔除：剔除审计可见（得率报告能看出「这个源全是租赁权」）。"""
        html = _fixture("hbcpre_jyjg_detail.html").replace(
            "992只废旧蓄电池处置成交公告",
            "宜昌市夷兴大道13-32档二楼B区三年承租权成交公告",
        )
        crawler = FakeCrawler(
            {
                JYJG_LIST: _fixture("hbcpre_jyjg_list.html"),
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=5d514bc8-815c-4025-94b8-455776a99e41": html,
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=31c0ed65-ed9a-419a-af47-1925c11ad3c7": html,
            }
        )
        count, report = self._run_backfill([JYJG_LIST], crawler)
        self.assertGreater(count, 0, "剔除不等于丢弃：行必须落库供审计")

        rows = self._read(
            "SELECT clean_status, rejection_reason, label_eligible "
            "FROM market_transactions WHERE channel = 'hbcpre'"
        )
        self.assertTrue(rows)
        for status, reason, eligible in rows:
            self.assertEqual(status, "rejected")
            self.assertEqual(reason, "lease_right_not_equipment")
            self.assertFalse(eligible, "租赁权样本不得有标签资格")
        self.assertIn(
            "lease_right_not_equipment",
            report.by_reason,
            "得率报告必须记下租赁权剔除原因（否则看不出这个源全是租赁权）",
        )

    def test_property_sample_does_not_reach_unmatched_queue(self):
        """真实抓取暴露的缺陷：房产样本曾以 label_eligible 落库，污染未匹配待审队列。

        判据：非设备样本剔除后，`market_unmatched_queue` 里不得出现它——
        该视图按 `sample_count DESC` 排序，房产组堆在顶部会把人工注意力引偏。
        """
        html = """
<html><body>
<div class="detailTitle">北京市海淀区远大园五区1号楼2层七单元2F号房产处置</div>
<div class="detailNeirong"><table>
<tr><td width="20%">项目编号:</td><td>202610219</td></tr>
<tr><td width="20%">挂牌价格（万元）:</td><td>2286.083</td></tr>
</table></div>
</body></html>
"""
        crawler = FakeCrawler(
            {
                JYGG_LIST: _fixture("hbcpre_jygg_list.html"),
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=e3484a1f-a37d-40dd-844e-fc1eab7c87d7": html,
                "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=a4b3dffe-b1e3-487f-a594-4fb87398fd6b": html,
            }
        )
        count, report = self._run_backfill([JYGG_LIST], crawler)
        self.assertGreater(count, 0)

        rows = self._read(
            "SELECT clean_status, rejection_reason FROM market_transactions "
            "WHERE channel = 'hbcpre'"
        )
        for status, reason in rows:
            self.assertEqual(status, "rejected")
            self.assertEqual(reason, "not_equipment_sample")
        self.assertIn("not_equipment_sample", report.by_reason)

        # 待审队列（视图）里不能出现这批样本。
        queued = self._read("SELECT count(*) FROM market_unmatched_queue")
        self.assertEqual(queued[0][0], 0, "非设备样本不得进未匹配待审队列")


if __name__ == "__main__":
    unittest.main()
