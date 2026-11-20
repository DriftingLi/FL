"""增量 daemon 的端到端契约测试（#2678 判据 1/2/3/4，对真 PG）。

钉的是「同一组件两种模式」这件事本身：daemon 每一轮必须**真的**走回补那条管线，
而不只是打日志。所以这里断言的是**副作用落到 PG 的事实**：
  - 判据 1：一轮 daemon 后新公告确实落库，且第二轮不重复落（判据 3）；
  - 判据 2：落库行带清洗状态、标签列，与 backfill 同口径；
  - 判据 4：抓取走同一个 CrawlClient（robots/限速/UA/退避），无平行实现。

抓取层仍用注入的 FakeCrawler（不碰网络）；robots/限速/退避的纯判定由
test_crawl_* 覆盖，这里只验「daemon 用的就是那个 crawler」。
"""

import os
import unittest
from pathlib import Path

DATABASE_URL = os.environ.get("DATABASE_URL")

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURES = REPO_ROOT / "collector" / "fixtures"

# 与 pg_fixture 保持同一组常量（那里才是唯一真相，这里只为少写一次 import 前缀）。
MASTER_KEY = "unit-test-master-key"

LIST_PAGE = "https://www1.rmfysszc.gov.cn/Agency/pmgg.aspx?fid=100"
DETAIL_A = "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/15001.shtml"
DETAIL_B = "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/15002.shtml"

LIST_HTML = f"""
<html><body><table><tbody>
<tr><td><a href="{DETAIL_A}" title="杭叉 CPCD30 周更公告">杭叉 CPCD30 周更公告</a></td>
    <td>某法院</td><td><span class="n_c_r">2026.11.18</span></td></tr>
<tr><td><a href="{DETAIL_B}" title="合力 CPCD50 周更公告">合力 CPCD50 周更公告</a></td>
    <td>某法院</td><td><span class="n_c_r">2026.11.19</span></td></tr>
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
    """注入的抓取层：按 URL 返回录好的 HTML，并记录每轮抓了哪些 URL。"""

    def __init__(self, pages: dict[str, str]) -> None:
        self._pages = pages
        self.fetched: list[str] = []
        self.log = type("Log", (), {"summary": lambda self_: "请求日志：测试桩"})()

    def fetch(self, url: str) -> str:
        self.fetched.append(url)
        if url not in self._pages:
            raise KeyError(f"未录制的 URL: {url}")
        return self._pages[url]

    def reset(self) -> None:
        self.fetched = []


class FakeClock:
    def __init__(self) -> None:
        self.now = 0.0
        self.sleeps: list[float] = []

    def time(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)
        self.now += seconds


@unittest.skipUnless(DATABASE_URL, "本机无 DATABASE_URL：真 PG 契约测试跳过（首跑在 CI）")
class DaemonE2EContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        from collector.tests import pg_fixture

        cls._pg = pg_fixture
        cls.conn, cls.schema, cls.sql = pg_fixture.create_schema(prefix="dm")
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

    def _stub(self):
        payload = (FIXTURES / "rmfysszc_extraction_stub.json").read_text(encoding="utf-8")

        def extractor(messages, config):
            assert config.model == "stub-model", config
            return payload

        return extractor

    def _pages(self):
        return {
            LIST_PAGE: LIST_HTML,
            DETAIL_A: _detail_html("杭叉 CPCD30 周更公告", "起拍价 42000 元"),
            DETAIL_B: _detail_html("合力 CPCD50 周更公告", "起拍价 88000 元"),
        }

    def test_one_cycle_lands_new_announcements_through_full_pipeline(self):
        """判据 1 + 2：daemon 一轮把新公告走全管线落库（带清洗状态与抽取版本）。"""
        from collector.cli import run_daemon_command

        self._clean_table()
        crawler = FakeCrawler(self._pages())
        clock = FakeClock()
        results = run_daemon_command(
            [LIST_PAGE],
            DATABASE_URL,
            self.schema,
            max_cycles=1,
            crawler=crawler,
            extractor=self._stub(),
            secret_key=MASTER_KEY,
            price_candidates=[],
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual(len(results), 1)
        self.assertFalse(results[0].failed, results[0].error)
        self.assertEqual(results[0].stored, 2, "一轮应落两条公告")

        rows = self._read(
            "SELECT source_url, clean_status, extraction_version "
            "FROM market_transactions ORDER BY source_url"
        )
        self.assertEqual(len(rows), 2)
        for _, status, version in rows:
            self.assertIn(status, ("raw", "cleaned", "rejected"))
            self.assertIsNotNone(version, "daemon 样本也必须经过抽取阶段")

    def test_second_cycle_is_idempotent(self):
        """判据 3：第二轮不重复落行，且已入库公告不再抓取。"""
        from collector.cli import run_daemon_command

        self._clean_table()
        crawler = FakeCrawler(self._pages())
        clock = FakeClock()
        results = run_daemon_command(
            [LIST_PAGE],
            DATABASE_URL,
            self.schema,
            max_cycles=2,
            crawler=crawler,
            extractor=self._stub(),
            secret_key=MASTER_KEY,
            price_candidates=[],
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual([r.stored for r in results], [2, 0], "第二轮不得重复落行")
        total = self._read("SELECT count(*) FROM market_transactions")[0][0]
        self.assertEqual(total, 2, "两轮后总行数仍是 2")
        # 第二轮只抓了列表页，没再抓详情页（幂等省请求）。
        detail_fetches = [u for u in crawler.fetched if "rm_xmdetail" in u]
        self.assertEqual(len(detail_fetches), 2, "详情页只应在第一轮被抓一次")

    def test_cycles_are_spaced_by_interval(self):
        """判据 1：周节奏——两轮之间补足一天（测试用 86400s 代替一周）。"""
        from collector.cli import run_daemon_command

        self._clean_table()
        crawler = FakeCrawler(self._pages())
        clock = FakeClock()
        run_daemon_command(
            [LIST_PAGE],
            DATABASE_URL,
            self.schema,
            interval_seconds=86400.0,
            max_cycles=2,
            crawler=crawler,
            extractor=self._stub(),
            secret_key=MASTER_KEY,
            price_candidates=[],
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual(clock.sleeps, [86400.0])

    def test_cycle_failure_is_isolated_and_reported(self):
        """判据 1（失败隔离）：整轮级错误（列表页抓不到）不终止 daemon，错误进 CycleResult。"""
        from collector.cli import run_daemon_command

        self._clean_table()
        # 列表页都抓不到 ⇒ 整轮抛 KeyError（区别于「单条详情失败」的部分成功）。
        crawler = FakeCrawler({
            DETAIL_A: _detail_html("杭叉 CPCD30 周更公告", "起拍价 42000 元"),
            # LIST_PAGE 故意不录制。
        })
        clock = FakeClock()
        results = run_daemon_command(
            [LIST_PAGE],
            DATABASE_URL,
            self.schema,
            max_cycles=2,
            crawler=crawler,
            extractor=self._stub(),
            secret_key=MASTER_KEY,
            price_candidates=[],
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual(len(results), 2, "失败后仍要跑下一轮")
        self.assertTrue(results[0].failed, "整轮抓取失败应记为失败")
        self.assertIn("pmgg", str(results[0].error), "错误应指向未录制的列表页")

    def test_uses_injected_crawler_for_all_requests(self):
        """判据 4：daemon 的全部请求都走同一个 crawler（robots/限速/UA/退避无平行实现）。"""
        from collector.cli import run_daemon_command

        self._clean_table()
        crawler = FakeCrawler(self._pages())
        clock = FakeClock()
        run_daemon_command(
            [LIST_PAGE],
            DATABASE_URL,
            self.schema,
            max_cycles=1,
            crawler=crawler,
            extractor=self._stub(),
            secret_key=MASTER_KEY,
            price_candidates=[],
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertIn(LIST_PAGE, crawler.fetched)
        self.assertIn(DETAIL_A, crawler.fetched)
        self.assertIn(DETAIL_B, crawler.fetched)

    def test_one_dead_url_does_not_starve_later_announcements(self):
        """毒丸饥饿：一条永久失败的详情 URL 不许挡住后面所有公告。

        一次性回补可以人工重跑/跳过；常驻 daemon 不能——若一条 404 卡在队头，
        每周都重撞它、后面所有新公告永远抓不到。实测过：8 周后只落 1 行。
        """
        from collector.cli import run_daemon_command

        self._clean_table()
        # DETAIL_A 会失败（未录制），DETAIL_B 正常——B 在 A 之后。
        crawler = FakeCrawler({
            LIST_PAGE: LIST_HTML,
            DETAIL_B: _detail_html("合力 CPCD50 周更公告", "起拍价 88000 元"),
        })
        clock = FakeClock()
        results = run_daemon_command(
            [LIST_PAGE],
            DATABASE_URL,
            self.schema,
            max_cycles=2,
            crawler=crawler,
            extractor=self._stub(),
            secret_key=MASTER_KEY,
            price_candidates=[],
            clock=clock.time,
            sleep=clock.sleep,
        )
        # B 必须落库——不能因为排在前面的 A 挂了就一起丢。
        urls = [r[0] for r in self._read("SELECT source_url FROM market_transactions")]
        self.assertIn(DETAIL_B, urls, "毒丸 URL 之后的公告仍必须被抓取并落库")
        self.assertNotIn(DETAIL_A, urls, "失败的 URL 不应落库")
        # 至少有一轮是「部分成功」而不是整体失败。
        self.assertGreaterEqual(sum(r.stored for r in results), 1)

    def test_partial_failure_reports_rows_it_did_store(self):
        """部分成功要如实上报：落库 1 行时不能只说「本轮失败 / 0 行」。"""
        from collector.cli import run_daemon_command

        self._clean_table()
        crawler = FakeCrawler({
            LIST_PAGE: LIST_HTML,
            DETAIL_B: _detail_html("合力 CPCD50 周更公告", "起拍价 88000 元"),
        })
        clock = FakeClock()
        results = run_daemon_command(
            [LIST_PAGE],
            DATABASE_URL,
            self.schema,
            max_cycles=1,
            crawler=crawler,
            extractor=self._stub(),
            secret_key=MASTER_KEY,
            price_candidates=[],
            clock=clock.time,
            sleep=clock.sleep,
        )
        self.assertEqual(results[0].stored, 1, "应如实上报本轮实际落库行数")
        self.assertEqual(
            results[0].failed_items, 1, "应如实上报本轮失败的 URL 数"
        )


if __name__ == "__main__":
    unittest.main()
