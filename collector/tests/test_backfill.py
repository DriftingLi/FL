"""回补批跑编排的单测（#2677 判据 1/3/5）。

判据原文：
  1. 一条命令完成历史回补（发现入口 = rmfysszc 公告列表/静态页）
  3. 回补样本走全管线，落库行带标签或清洗状态
  5. 已入库公告幂等（重跑不重复落行）

第一性原理：回补 = 「发现 URL 集合」×「对每个 URL 跑一遍既有 batch 管线」。
本票不重写管线（#2672–#2676 已建），只加「发现」与「编排」两层：
  - 发现层产出 `DiscoveredAnnouncement(url, publish_date, title)` 列表，不碰 PG；
  - 编排层按 URL 逐个走 run_batch 的同一套阶段，用 source_url 判重实现幂等。

TDD 红：先于实现写下，钉住：
  - 列表页 HTML → 公告条目（URL/标题/发布日期），相对链接补全为绝对；
  - 年份过滤（2015–2026）只留区间内公告；
  - 已入库 URL 跳过（幂等），不重复落行；
  - 得率报告：各清洗状态计数 + 各剔除原因占比，无硬门槛。
"""

import unittest

from collector.backfill import (
    DiscoveredAnnouncement,
    build_yield_report,
    filter_by_year,
    parse_announcement_list,
    select_new_announcements,
)

LIST_HTML = """
<html><body>
<table><tbody>
<tr class="listtr">
  <td><span class="n_c_l"><a
    href="https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/13184763.shtml"
    title="义乌市人民法院关于不动产的公告">义乌市人民法院关于不动产的公告</a></span></td>
  <td><span title="义乌市人民法院">义乌市人民法院</span></td>
  <td><span class="n_c_r">2026.11.20</span></td>
</tr>
<tr class="listtr">
  <td><span class="n_c_l"><a
    href="/statichtml/rm_xmdetail/196036.shtml"
    title="2015 年叉车公告">2015 年叉车公告</a></span></td>
  <td><span title="某法院">某法院</span></td>
  <td><span class="n_c_r">2015.04.24</span></td>
</tr>
<tr class="listtr">
  <td><span class="n_c_l"><a
    href="https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/1241367-1.shtml"
    title="2018 年叉车公告">2018 年叉车公告</a></span></td>
  <td><span title="某法院">某法院</span></td>
  <td><span class="n_c_r">2018.05.11</span></td>
</tr>
</tbody></table>
</body></html>
"""


class ParseAnnouncementListTest(unittest.TestCase):
    def test_extracts_url_title_and_publish_date(self):
        items = parse_announcement_list(LIST_HTML)
        self.assertEqual(len(items), 3)
        first = items[0]
        self.assertEqual(
            first.url,
            "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/13184763.shtml",
        )
        self.assertIn("不动产", first.title)
        self.assertEqual(first.publish_date, "2026-11-20")

    def test_relative_href_is_absolutized(self):
        items = parse_announcement_list(LIST_HTML)
        relative = [i for i in items if "196036" in i.url][0]
        self.assertTrue(relative.url.startswith("https://www.rmfysszc.gov.cn/"))

    def test_deduplicates_same_url(self):
        html = LIST_HTML + LIST_HTML
        items = parse_announcement_list(html)
        urls = [i.url for i in items]
        self.assertEqual(len(urls), len(set(urls)))

    def test_empty_html_yields_nothing(self):
        self.assertEqual(parse_announcement_list(""), [])


class FilterByYearTest(unittest.TestCase):
    def test_keeps_only_requested_year_range(self):
        items = [
            DiscoveredAnnouncement("https://x.test/1", "2015 公告", "2015-04-24"),
            DiscoveredAnnouncement("https://x.test/2", "2018 公告", "2018-05-11"),
            DiscoveredAnnouncement("https://x.test/3", "2026 公告", "2026-11-20"),
            DiscoveredAnnouncement("https://x.test/4", "无日期公告", ""),
        ]
        kept = filter_by_year(items, start_year=2015, end_year=2026)
        self.assertEqual([i.url for i in kept], ["https://x.test/1", "https://x.test/2", "https://x.test/3"])

    def test_out_of_range_dropped(self):
        items = [
            DiscoveredAnnouncement("https://x.test/1", "2014 公告", "2014-12-31"),
            DiscoveredAnnouncement("https://x.test/2", "2027 公告", "2027-01-01"),
        ]
        self.assertEqual(filter_by_year(items, start_year=2015, end_year=2026), [])


class IdempotencyTest(unittest.TestCase):
    def test_already_stored_urls_are_skipped(self):
        discovered = [
            DiscoveredAnnouncement("https://x.test/a", "A", "2020-01-01"),
            DiscoveredAnnouncement("https://x.test/b", "B", "2020-01-02"),
            DiscoveredAnnouncement("https://x.test/c", "C", "2020-01-03"),
        ]
        new = select_new_announcements(discovered, existing_urls={"https://x.test/a", "https://x.test/c"})
        self.assertEqual([i.url for i in new], ["https://x.test/b"])

    def test_duplicate_urls_inside_batch_are_collapsed(self):
        discovered = [
            DiscoveredAnnouncement("https://x.test/a", "A", "2020-01-01"),
            DiscoveredAnnouncement("https://x.test/a", "A 重发", "2020-01-01"),
        ]
        new = select_new_announcements(discovered, existing_urls=set())
        self.assertEqual(len(new), 1)


class YieldReportTest(unittest.TestCase):
    def test_counts_by_clean_status_and_rejection_reasons(self):
        rows = [
            {"clean_status": "cleaned", "rejection_reason": None},
            {"clean_status": "cleaned", "rejection_reason": None},
            {"clean_status": "rejected", "rejection_reason": "bulk_lot_no_per_unit"},
            {"clean_status": "rejected", "rejection_reason": "bulk_lot_no_per_unit"},
            {"clean_status": "rejected", "rejection_reason": "negotiable_price"},
            {"clean_status": "raw", "rejection_reason": None},
        ]
        report = build_yield_report(rows)
        self.assertEqual(report.total, 6)
        self.assertEqual(report.by_status, {"cleaned": 2, "rejected": 3, "raw": 1})
        self.assertEqual(report.by_reason["bulk_lot_no_per_unit"], 2)
        self.assertEqual(report.by_reason["negotiable_price"], 1)
        # 得率 = cleaned / total，记录为事实，不设门槛。
        self.assertAlmostEqual(report.yield_rate, 2 / 6)

    def test_yield_rate_is_none_when_no_rows(self):
        report = build_yield_report([])
        self.assertEqual(report.total, 0)
        self.assertIsNone(report.yield_rate)

    def test_report_renders_reason_share(self):
        rows = [
            {"clean_status": "rejected", "rejection_reason": "bulk_lot_no_per_unit"},
            {"clean_status": "rejected", "rejection_reason": "negotiable_price"},
            {"clean_status": "cleaned", "rejection_reason": None},
        ]
        text = build_yield_report(rows).render()
        self.assertIn("bulk_lot_no_per_unit", text)
        self.assertIn("得率", text)


if __name__ == "__main__":
    unittest.main()
