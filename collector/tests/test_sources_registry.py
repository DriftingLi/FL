"""采集面注册表与结构化直取判定的单测（#2679 / #2680 的共享 seam）。

注册表是「加采集面 = 加一个模块文件」这个并行开发机制的实现点。它必须：
  - 每个已登记采集面的 channel / origins / markers 自洽（否则 robots 读错 host）；
  - 按 URL 能精确识别（最长片段胜，识别不出即报错，不猜不回落）；
  - 新模块被自动发现（改 `__init__.py` 才算失败）。

结构化直取的判定（`apply_structured`）单列，因为它是「跳过 LLM」这条捷径的闸门：
价格门槛不能因采集面而放宽，否则结构化样本会绕过「无价格不算标签」的硬约束。
"""

import json
import unittest

from collector.extract.decisions import (
    LOW_CONFIDENCE_THRESHOLD,
    claimed_low_confidence_reason,
)
from collector.extract.service import STRUCTURED_VERSION, apply_structured
from collector.models import RawSample
from collector.sources import all_sources, resolve_source, source_for_channel


class RegistryTest(unittest.TestCase):
    def test_all_three_phase_one_sources_are_registered(self):
        channels = {spec.channel for spec in all_sources()}
        self.assertEqual(
            channels, {"rmfysszc", "jd", "gpai"},
            "一期采集面 = rmfysszc（既有）+ 京东 + 公拍网；产权交易所由 #2680 另加",
        )

    def test_every_source_has_origins_and_markers(self):
        for spec in all_sources():
            with self.subTest(channel=spec.channel):
                self.assertTrue(spec.origins, "没 origins 就读不了 robots")
                self.assertTrue(spec.url_markers, "没 markers 就无法按 URL 识别")
                for origin in spec.origins:
                    self.assertRegex(origin, r"^https?://[^/]+$", "origin 不能带路径")

    def test_only_rmfysszc_is_unstructured(self):
        """结构化直取是例外路径：谁走它必须是有意的、可列举的。"""
        structured = {s.channel for s in all_sources() if s.structured}
        self.assertEqual(structured, {"jd", "gpai"})

    def test_structured_sources_keep_undated(self):
        """结构化平台没有发布日期字段；若不放行无日期条目，回补会静默落 0 行。"""
        for spec in all_sources():
            if spec.structured:
                with self.subTest(channel=spec.channel):
                    self.assertTrue(spec.keep_undated)

    def test_resolve_jd_urls(self):
        self.assertEqual(
            resolve_source("https://paimai.jd.com/310055258").channel, "jd"
        )
        self.assertEqual(
            resolve_source("https://paimai.jd.com/notice/11810994").channel, "jd"
        )

    def test_resolve_gpai_urls(self):
        self.assertEqual(
            resolve_source("https://www.gpai.net/sf/item2.do?Web_Item_ID=53260").channel,
            "gpai",
        )
        self.assertEqual(
            resolve_source("https://zc.gpai.net/zc/detail?id=8817149").channel, "gpai"
        )

    def test_resolve_rmfysszc_urls(self):
        self.assertEqual(
            resolve_source(
                "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/13184763.shtml"
            ).channel,
            "rmfysszc",
        )
        self.assertEqual(
            resolve_source("https://www1.rmfysszc.gov.cn/Agency/pmgg.aspx?fid=100").channel,
            "rmfysszc",
        )

    def test_unknown_url_raises_with_known_list(self):
        with self.assertRaises(ValueError) as ctx:
            resolve_source("https://example.com/whatever")
        message = str(ctx.exception)
        self.assertIn("rmfysszc", message)
        self.assertIn("jd", message)

    def test_source_for_channel_rejects_unknown(self):
        with self.assertRaises(ValueError):
            source_for_channel("taobao")

    def test_longest_marker_wins(self):
        """更具体的频道路径优先于域名（否则 zc 会被 www 的规则抢走）。"""
        # gpai 的 markers 里 "gpai.net/sf/" 比 "zc.gpai.net/" 不重叠，这里验机制本身：
        # resolve_source 选最长匹配片段。
        self.assertEqual(
            resolve_source("https://www.gpai.net/sf/item2.do?Web_Item_ID=1").channel,
            "gpai",
        )


def _structured_sample(**overrides) -> RawSample:
    base = dict(
        channel="jd",
        source_url="https://paimai.jd.com/1",
        raw_description={"title": "叉车"},
        raw_price=15000.0,
        raw_price_kind="deal",
        structured_specs={"brand": "林德", "tonnage": 2.0},
        field_confidence={"raw_price": 1.0, "tonnage": 0.9},
    )
    base.update(overrides)
    return RawSample(**base)


class ApplyStructuredTest(unittest.TestCase):
    def test_price_with_max_confidence_is_label_eligible(self):
        updated, review = apply_structured(_structured_sample())
        self.assertTrue(updated.label_eligible)
        self.assertEqual(updated.extraction_version, STRUCTURED_VERSION)
        self.assertEqual(review, [], "价格取到且置信达标 ⇒ 不入队")

    def test_missing_price_is_not_label_eligible(self):
        """价格门槛不能因采集面而放宽——无价格就是无标签。"""
        updated, review = apply_structured(
            _structured_sample(raw_price=None, field_confidence={})
        )
        self.assertFalse(updated.label_eligible)
        self.assertIn("price_extraction_failed", [r for r, _ in review])

    def test_low_price_confidence_is_not_label_eligible(self):
        updated, review = apply_structured(
            _structured_sample(field_confidence={"raw_price": 0.5})
        )
        self.assertFalse(updated.label_eligible)
        self.assertIn("price_extraction_failed", [r for r, _ in review])

    def test_absent_spec_fields_do_not_trigger_review(self):
        """结构化源不提供的字段（如使用小时）不算抽取失败——只有价格是硬判据。"""
        updated, review = apply_structured(
            _structured_sample(field_confidence={"raw_price": 1.0})
        )
        self.assertTrue(updated.label_eligible)
        self.assertNotIn("low_confidence", [r for r, _ in review])

    def test_claimed_but_low_confidence_field_is_queued(self):
        """声称取到但置信不足的字段仍要进抽检队列。"""
        _, review = apply_structured(
            _structured_sample(field_confidence={"raw_price": 1.0, "tonnage": 0.5})
        )
        self.assertIn("low_confidence", [r for r, _ in review])

    def test_structured_version_differs_from_llm_prompt_version(self):
        from collector.extract.prompt import PROMPT_VERSION

        self.assertNotEqual(STRUCTURED_VERSION, PROMPT_VERSION)


class ClaimedLowConfidenceTest(unittest.TestCase):
    def test_only_present_fields_are_examined(self):
        detail = claimed_low_confidence_reason({"raw_price": 1.0, "tonnage": 0.5})
        payload = json.loads(detail)
        self.assertEqual(payload["low_confidence_fields"], ["tonnage"])

    def test_missing_key_fields_are_not_reported(self):
        """与 LLM 口径的关键差别：缺席 ≠ 抽取失败。"""
        self.assertIsNone(
            claimed_low_confidence_reason({"raw_price": 1.0}),
            "brand/model/year 等字段缺席是源站不提供，不该进抽检",
        )

    def test_threshold_is_shared_with_llm_path(self):
        detail = claimed_low_confidence_reason(
            {"raw_price": LOW_CONFIDENCE_THRESHOLD - 0.01}
        )
        self.assertIsNotNone(detail)


if __name__ == "__main__":
    unittest.main()
