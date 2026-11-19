"""LLM 抽取的纯判定单测（不依赖 PG / 网络）。

TDD 红：这些用例先于实现写下，钉住 #2673 的三条判定口径：
  - 低置信字段（< 0.75）→ 样本标记进抽检队列；
  - 价格字段抽取失败 → 无标签资格；
  - 抽取版本与 prompt 版本同源（版本化）。
判定规则（阈值 0.75、关键字段集）由用户在本会话确认，落进具名常量并写进 ADR-0097。
"""

import unittest

from collector.extract.decisions import (
    KEY_FIELDS,
    LOW_CONFIDENCE_THRESHOLD,
    Decision,
    decide,
)
from collector.extract.prompt import PROMPT_VERSION, build_messages, parse_response


class ExtractDecisionTest(unittest.TestCase):
    def test_high_confidence_sample_needs_no_review_and_is_label_eligible(self):
        result = decide(
            field_confidence={
                "brand": 0.97, "model": 0.95, "series": 0.93,
                "tonnage": 0.91, "year": 0.93, "engine_hours": 0.88,
                "raw_price": 0.96,
            },
            raw_price=42000.0,
        )
        self.assertEqual(result, Decision(needs_review=False, label_eligible=True))
        self.assertEqual(LOW_CONFIDENCE_THRESHOLD, 0.75)
        self.assertIn("raw_price", KEY_FIELDS)

    def test_low_confidence_field_marks_sample_for_review(self):
        # series 置信度 0.62 < 0.75：即便价格抽到了，也要进抽检队列。
        result = decide(
            field_confidence={"brand": 0.97, "series": 0.62, "raw_price": 0.96},
            raw_price=42000.0,
        )
        self.assertTrue(result.needs_review)
        self.assertTrue(result.label_eligible)

    def test_missing_confidence_for_key_field_marks_for_review(self):
        # 关键字段缺置信度 = 不可信，从严进抽检。
        result = decide(
            field_confidence={"raw_price": 0.96},
            raw_price=42000.0,
        )
        self.assertTrue(result.needs_review)

    def test_price_extraction_failure_removes_label_eligibility(self):
        result = decide(
            field_confidence={"brand": 0.97, "model": 0.95, "raw_price": 0.41},
            raw_price=None,
        )
        self.assertFalse(result.label_eligible)
        self.assertTrue(result.needs_review)  # 价格也是关键字段，低置信/缺失 ⇒ 抽检


class PromptVersionTest(unittest.TestCase):
    def test_prompt_is_versioned_and_parses_recorded_response(self):
        self.assertTrue(PROMPT_VERSION.startswith("v"))
        messages = build_messages("公告正文样例")
        self.assertIn(PROMPT_VERSION, messages[0]["content"])
        self.assertEqual(messages[1]["content"], "公告正文样例")

        import json

        recorded = json.dumps({
            "structured_specs": {"brand": "杭叉", "model": "CPCD30"},
            "field_confidence": {"brand": 0.97, "model": 0.95},
            "raw_price": 42000,
            "raw_price_kind": "listing",
            "appraisal_price": None,
        }, ensure_ascii=False)
        parsed = parse_response(recorded)
        self.assertEqual(parsed["raw_price"], 42000)
        self.assertEqual(parsed["field_confidence"]["brand"], 0.97)

    def test_parse_response_rejects_non_json(self):
        with self.assertRaises(ValueError):
            parse_response("这不是 JSON")


if __name__ == "__main__":
    unittest.main()
