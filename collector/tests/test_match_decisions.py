"""校准专用匹配器纯判定单测（#2675，不依赖 PG / 网络）。

TDD 红：这些用例先于实现写下，钉住 #2675 的判定口径：
  - 7→4→3 字段分层降级，各层 fixture 覆盖；
  - 多匹配取原价中位数（不是评估路径的取最高价）；
  - 匹配层级 + 原价区间入表，区间宽度进可信度；
  - 残值率 = 成交价 ÷ 匹配原价；未匹配留底、无标签、不进校准集；
  - 匹配是纯函数、版本化，同输入同输出，不改入参。
"""

import json
import unittest
from pathlib import Path

from collector.match import MATCH_VERSION, match
from collector.models import RawSample

FIXTURES = Path(__file__).resolve().parents[1] / "fixtures"


def _sample(extraction, *, price=42000.0, url="https://example.test/match"):
    return RawSample(
        channel="rmfysszc",
        source_url=url,
        raw_description={"title": "公告", "body": "正文"},
        structured_specs=dict(extraction),
        raw_price=price,
        raw_price_kind="deal",
        label_eligible=True,
    )


def _candidate(brand, vehicle_type, series, tonnage, price, **rest):
    row = {
        "brand": brand, "vehicle_type": vehicle_type, "series": series,
        "tonnage": tonnage, "original_price": price,
        "config_type": rest.get("config_type", "标准"),
        "mast_type": rest.get("mast_type", "二节"),
        "mast_height_mm": rest.get("mast_height_mm", 3000),
    }
    return row


class MatchLayeringTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cases = json.loads((FIXTURES / "match_cases.json").read_text(encoding="utf-8"))

    def _candidates(self, case):
        return [
            _candidate(
                c["brand"], c["vehicle_type"], c["series"], c["tonnage"],
                c["original_price"], **{k: v for k, v in c.items()
                                        if k in ("config_type", "mast_type", "mast_height_mm")}
            )
            for c in case["candidates"]
        ]

    def test_three_layering_levels(self):
        for name, case in self.cases.items():
            if not name.startswith("layer"):
                continue
            with self.subTest(case=name):
                sample = _sample(case["specs"], price=case["price"])
                result = match(sample, self._candidates(case))
                self.assertEqual(result.match_level, case["expect_level"], name)
                self.assertIsNotNone(result.matched_original_price, name)

    def test_multi_match_takes_median_not_max(self):
        case = self.cases["median"]
        result = match(_sample(case["specs"], price=case["price"]), self._candidates(case))
        # 候选 40000/50000/60000：中位数 50000，最大值 60000 —— 必须是中位数。
        self.assertEqual(result.matched_original_price, 50000.0)
        self.assertNotEqual(result.matched_original_price, 60000.0)

    def test_even_match_median_is_mean_of_two_middle(self):
        case = self.cases["median_even"]
        result = match(_sample(case["specs"], price=case["price"]), self._candidates(case))
        self.assertEqual(result.matched_original_price, 50000.0)  # (40000+60000)/2

    def test_range_and_confidence_recorded(self):
        case = self.cases["median"]
        result = match(_sample(case["specs"], price=case["price"]), self._candidates(case))
        rng = result.matched_original_range
        self.assertEqual(rng["min"], 40000.0)
        self.assertEqual(rng["max"], 60000.0)
        self.assertEqual(rng["count"], 3)
        self.assertEqual(rng["width"], 20000.0)
        # 区间宽度进可信度：1/(1+20000/50000)=0.714286，落在 field_confidence 上。
        self.assertAlmostEqual(result.field_confidence["matched_original_price"],
                               1 / (1 + 20000 / 50000), places=5)


class LabelTest(unittest.TestCase):
    def test_residual_rate_is_price_div_matched_original(self):
        sample = _sample(
            {"brand": "杭叉", "vehicle_type": "CPCD30", "series": "CPCD", "tonnage": 3.0},
            price=42000.0,
        )
        candidates = [_candidate("杭叉", "CPCD30", "CPCD", 3.0, 50000.0)]
        result = match(sample, candidates)
        self.assertAlmostEqual(result.residual_rate_label, 42000 / 50000, places=6)

    def test_unmatched_is_kept_no_label_no_calibration(self):
        case = json.loads((FIXTURES / "match_cases.json").read_text(encoding="utf-8"))["unmatched"]
        sample = _sample(case["specs"], price=case["price"])
        result = match(sample, [])
        self.assertIsNone(result.match_level)
        self.assertIsNone(result.matched_original_price)
        self.assertIsNone(result.residual_rate_label)
        self.assertEqual(result.matched_original_range, {})
        # 未匹配不改 label_eligible（价格抽取仍成功），标签空 ⇒ 不进校准集由 partial index 保证。
        self.assertTrue(result.label_eligible)

    def test_no_eval_max_price_semantics(self):
        """两个候选只命中 3 字段层、价格 40000/90000 —— 取中位数 65000，不是 90000。"""
        sample = _sample({"brand": "杭叉", "vehicle_type": "CPCD30", "tonnage": 3.0}, price=42000.0)
        candidates = [
            _candidate("杭叉", "CPCD30", "X1", 3.0, 40000.0),
            _candidate("杭叉", "CPCD30", "X2", 3.0, 90000.0),
        ]
        result = match(sample, candidates)
        self.assertEqual(result.matched_original_price, 65000.0)

    def test_wrong_value_candidates_are_excluded(self):
        """列齐全但规格值不同的候选绝不能命中——否则真表上所有行都会误匹配。

        这是 code-review 抓到的关键 bug 回归：_match_at 早期只检查键存在、不比值，
        会把任意品牌/吨位的原价当中位数。
        """
        sample = _sample(
            {"brand": "杭叉", "vehicle_type": "CPCD30", "series": "CPCD", "tonnage": 3.0},
            price=42000.0,
        )
        # 三个候选列都齐全，但品牌/车型/吨位全不对——必须全部排除。
        candidates = [
            _candidate("合力", "CPCD50", "CPCD", 5.0, 1000000.0),
            _candidate("丰田", "7F", "7F", 7.0, 2000000.0),
        ]
        result = match(sample, candidates)
        self.assertIsNone(result.matched_original_price)
        self.assertIsNone(result.match_level)
        self.assertIsNone(result.residual_rate_label)
        self.assertEqual(result.matched_original_range, {})

    def test_only_matching_value_candidate_is_used(self):
        """混入一个错值候选 + 一个对值候选 — 只用对值那个，不被高价污染。"""
        sample = _sample(
            {"brand": "杭叉", "vehicle_type": "CPCD30", "series": "CPCD", "tonnage": 3.0},
            price=42000.0,
        )
        candidates = [
            _candidate("合力", "CPCD50", "CPCD", 5.0, 900000.0),  # 错值
            _candidate("杭叉", "CPCD30", "CPCD", 3.0, 50000.0),   # 对值
        ]
        result = match(sample, candidates)
        self.assertEqual(result.matched_original_price, 50000.0)
        self.assertEqual(result.match_level, "brand_vehicle_series_tonnage")
        self.assertAlmostEqual(result.residual_rate_label, 42000 / 50000, places=6)

    def test_vehicle_type_none_falls_back_to_model(self):
        """vehicle_type 显式为 None 时应回退到 model（早期 .get(k, default) 不会回退）。"""
        sample = _sample(
            {"brand": "杭叉", "vehicle_type": None, "model": "CPCD30", "tonnage": 3.0},
            price=42000.0,
        )
        candidates = [_candidate("杭叉", "CPCD30", "CPCD", 3.0, 50000.0)]
        result = match(sample, candidates)
        self.assertEqual(result.matched_original_price, 50000.0)


class MatchPurityTest(unittest.TestCase):
    def test_versioned_pure_deterministic_no_input_mutation(self):
        self.assertTrue(MATCH_VERSION.startswith("v"))
        case = json.loads((FIXTURES / "match_cases.json").read_text(encoding="utf-8"))["median"]
        sample = _sample(case["specs"], price=case["price"])
        candidates = [
            _candidate(c["brand"], c["vehicle_type"], c["series"], c["tonnage"], c["original_price"])
            for c in case["candidates"]
        ]
        specs_before = dict(sample.structured_specs)
        a = match(sample, candidates)
        b = match(sample, candidates)
        self.assertEqual(a, b)
        self.assertEqual(a.match_version, MATCH_VERSION)
        self.assertEqual(sample.structured_specs, specs_before)


if __name__ == "__main__":
    unittest.main()
