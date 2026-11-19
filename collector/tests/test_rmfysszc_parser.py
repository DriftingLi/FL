"""录制的 rmfysszc 公告解析单测（纯函数，不依赖 PG）。

TDD 红：本测先于实现写下。解析 seam 由 spec #2671 定为「fixture HTML 进 → raw 样本出」，
抽取/清洗/匹配尚未接入，故这里只断言原始留底字段（渠道/来源 URL/原始描述 jsonb 内容）。
"""

import unittest
from pathlib import Path

from collector.models import RawSample
from collector.parsers.rmfysszc import parse_announcement

FIXTURE = Path(__file__).resolve().parents[1] / "fixtures" / "rmfysszc_announcement.html"


class ParseAnnouncementTest(unittest.TestCase):
    def test_parses_recorded_fixture_into_one_raw_sample(self):
        html = FIXTURE.read_text(encoding="utf-8")
        samples = parse_announcement(html)

        self.assertEqual(len(samples), 1)
        sample = samples[0]
        self.assertIsInstance(sample, RawSample)
        self.assertEqual(sample.channel, "rmfysszc")
        self.assertEqual(
            sample.source_url,
            "https://www.rmfysszc.com/auction/notice/2026/11001.html",
        )
        self.assertEqual(sample.raw_description["title"], "杭叉 CPCD30 叉车公开拍卖公告")
        self.assertEqual(sample.raw_description["publish_date"], "2026-11-18")
        self.assertIn("起拍价 42000 元", sample.raw_description["body"])

    def test_empty_document_yields_no_samples(self):
        self.assertEqual(parse_announcement("<html><body></body></html>"), [])


if __name__ == "__main__":
    unittest.main()
