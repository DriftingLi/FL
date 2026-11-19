"""清洗规则纯判定单测（#2674，不依赖 PG / 网络）。

TDD 红：这些用例先于实现写下，钉住 #2674 的判定口径：
  - 8 类噪声 fixture 各落正确清洗状态（剔除原因 / 溢价标记降权）；
  - 买受人身份（姓名 / 竞买号）从结构化字段与原文留底剥离；
  - 车商去重三形态（图哈希 / 描述相似度 / 同公司跨城）各命中；
  - 清洗是纯函数、版本化，同输入同输出，不改入参。
"""

import json
import unittest
from pathlib import Path

from collector.models import RawSample
from collector.clean.rules import CLEAN_VERSION, clean

FIXTURES = Path(__file__).resolve().parents[1] / "fixtures"


def _sample_from(extraction, *, url="https://example.test/default", body="普通公告正文"):
    return RawSample(
        channel="rmfysszc",
        source_url=url,
        raw_description={"title": "标题", "publish_date": "2026-01-01", "body": body},
        structured_specs=dict(extraction.get("structured_specs") or {}),
        field_confidence=dict(extraction.get("field_confidence") or {}),
        raw_price=extraction.get("raw_price"),
        raw_price_kind=extraction.get("raw_price_kind"),
        appraisal_price=extraction.get("appraisal_price"),
        label_eligible=extraction.get("raw_price") is not None,
    )


class CleanNoiseTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cases = json.loads((FIXTURES / "clean_noise_cases.json").read_text(encoding="utf-8"))

    def test_eight_noise_fixtures_land_correct_clean_status(self):
        for name, case in self.cases.items():
            if name == "schema_version":
                continue
            with self.subTest(case=name):
                sample = _sample_from(case["extraction"], url=f"https://example.test/{name}")
                cleaned, review = clean(sample)
                exp = case["expect"]
                self.assertEqual(cleaned.clean_status, exp["clean_status"], name)
                if "premium_anomaly" in exp:
                    self.assertEqual(cleaned.premium_anomaly, exp["premium_anomaly"], name)
                if "reason_contains" in exp:
                    self.assertIsNotNone(cleaned.rejection_reason, name)
                    self.assertIn(exp["reason_contains"], cleaned.rejection_reason, name)
                if "label_eligible" in exp:
                    self.assertEqual(cleaned.label_eligible, exp["label_eligible"], name)
                if "review_reason" in exp:
                    self.assertIn(exp["review_reason"], [r[0] for r in review], name)

    def test_premium_is_marked_not_rejected(self):
        case = self.cases["premium"]
        cleaned, review = clean(_sample_from(case["extraction"], url="https://example.test/premium"))
        self.assertEqual(cleaned.clean_status, "cleaned")
        self.assertTrue(cleaned.premium_anomaly)
        self.assertIsNone(cleaned.rejection_reason)
        self.assertIn("premium_anomaly", [r[0] for r in review])

    def test_rejected_sample_keeps_audit_reason(self):
        case = self.cases["bulk_lot"]
        cleaned, _ = clean(_sample_from(case["extraction"], url="https://example.test/bulk"))
        self.assertEqual(cleaned.clean_status, "rejected")
        self.assertTrue(cleaned.rejection_reason)


class IdentityStripTest(unittest.TestCase):
    def test_buyer_identity_stripped_from_specs_and_raw_description(self):
        sample = RawSample(
            channel="rmfysszc",
            source_url="https://example.test/identity",
            raw_description={
                "title": "成交确认书",
                "publish_date": "2026-01-01",
                "body": "买受人 张三（竞买号 A1234）以 42000 元竞得",
                "buyer_name": "张三",
                "bidder_no": "A1234",
            },
            structured_specs={"brand": "杭叉", "buyer_name": "张三", "bidder_no": "A1234", "id_no": "310..."},
            field_confidence={"brand": 0.95, "raw_price": 0.95},
            raw_price=42000,
            raw_price_kind="deal",
            appraisal_price=40000,
            label_eligible=True,
        )
        cleaned, _ = clean(sample)
        for key in ("buyer_name", "bidder_no", "id_no"):
            self.assertNotIn(key, cleaned.structured_specs)
            self.assertNotIn(key, cleaned.raw_description)
        # 行内身份值也必须脱敏（PIPL：留底亦不含买受人身份）；标签与非身份正文保留。
        body = cleaned.raw_description["body"]
        self.assertNotIn("张三", body)
        self.assertNotIn("A1234", body)
        self.assertIn("买受人", body)
        self.assertIn("竞买号", body)
        self.assertIn("以 42000 元竞得", body)
        self.assertEqual(cleaned.clean_status, "cleaned")


class DedupTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cases = json.loads((FIXTURES / "clean_dedup_cases.json").read_text(encoding="utf-8"))

    def _existing(self, case):
        out = []
        for row in case["existing"]:
            out.append(RawSample(
                channel="rmfysszc",
                source_url=row["source_url"],
                raw_description=dict(row["raw_description"]),
                structured_specs=dict(row["structured_specs"]),
            ))
        return out

    def test_three_dedup_forms_each_hit(self):
        expectations = {
            "image_hash_case": "image_hash",
            "description_similarity_case": "description_similarity",
            "same_company_cross_city_case": "same_company_cross_city",
        }
        for case_name, form in expectations.items():
            with self.subTest(case=case_name):
                case = self.cases[case_name]
                sample = _sample_from(case["new"], url=f"https://example.test/{case_name}",
                                      body=case["new"].get("body", "普通公告正文"))
                cleaned, _ = clean(sample, existing=self._existing(case))
                self.assertEqual(cleaned.clean_status, "rejected", case_name)
                self.assertIn(form, cleaned.rejection_reason, case_name)

    def test_no_false_positive_on_distinct_posts(self):
        case = self.cases["image_hash_case"]
        sample = _sample_from(case["new"], url="https://example.test/unique")
        cleaned, _ = clean(sample, existing=[])
        self.assertNotEqual(cleaned.clean_status, "rejected")


class CleanPurityTest(unittest.TestCase):
    def test_versioned_pure_deterministic_no_input_mutation(self):
        self.assertTrue(CLEAN_VERSION.startswith("v"))
        case = json.loads((FIXTURES / "clean_noise_cases.json").read_text(encoding="utf-8"))["clean"]
        sample = _sample_from(case["extraction"], url="https://example.test/pure")
        specs_before = dict(sample.structured_specs)
        raw_before = dict(sample.raw_description)
        a, _ = clean(sample)
        b, _ = clean(sample)
        self.assertEqual(a, b)
        # 版本必须真的落到产出结果上，不能只是导出一个装饰性常量。
        self.assertEqual(a.clean_version, CLEAN_VERSION)
        self.assertEqual(sample.structured_specs, specs_before)
        self.assertEqual(sample.raw_description, raw_before)


if __name__ == "__main__":
    unittest.main()
