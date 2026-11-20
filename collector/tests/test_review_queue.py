"""未匹配待审队列聚合 + 重匹配回收标签的单测（#2676）。

TDD 红：先于实现写下，钉住 #2676 的判定口径：
  - 未匹配样本按 品牌×型号×吨位 聚合成可查对象（样本数 + 成交价区间）；
  - 聚合只收「未匹配」行：已匹配、清洗被剔除的都不算待审；
  - 补原价条目后重跑匹配可回收标签，且与首次匹配同口径（层级/区间/标签）。
"""

import unittest

from collector.review import (
    UNMATCHED_GROUP_COLUMNS,
    group_unmatched,
    unmatched_group_key,
)
from collector.models import RawSample


def _sample(brand="杭叉", model="CPCD30", tonnage=3.0, price=42000.0, *, url="https://x.test/1", **kw):
    specs = {}
    if brand is not None:
        specs["brand"] = brand
    if model is not None:
        specs["model"] = model
    if tonnage is not None:
        specs["tonnage"] = tonnage
    return RawSample(
        channel="rmfysszc",
        source_url=url,
        raw_description={"title": "公告", "body": "正文"},
        structured_specs=specs,
        raw_price=price,
        raw_price_kind="deal",
        label_eligible=True,
        **kw,
    )


class GroupKeyTest(unittest.TestCase):
    def test_group_key_is_brand_model_tonnage(self):
        self.assertEqual(unmatched_group_key(_sample()), ("杭叉", "CPCD30", 3.0))

    def test_missing_spec_degrades_to_none_parts(self):
        key = unmatched_group_key(_sample(brand=None, model=None, tonnage=None))
        self.assertEqual(key, (None, None, None))


class GroupUnmatchedTest(unittest.TestCase):
    def test_groups_by_key_with_count_and_price_range(self):
        samples = [
            _sample(price=40000.0, url="https://x.test/1"),
            _sample(price=50000.0, url="https://x.test/2"),
            _sample(price=60000.0, url="https://x.test/3"),
            _sample(brand="合力", model="CPCD50", tonnage=5.0, price=80000.0, url="https://x.test/4"),
        ]
        groups = group_unmatched(samples)
        self.assertEqual(len(groups), 2)
        by_key = {
            (g["brand"], g["model"], g["tonnage"]): g for g in groups
        }
        hc = by_key[("杭叉", "CPCD30", 3.0)]
        self.assertEqual(hc["sample_count"], 3)
        self.assertEqual(hc["price_min"], 40000.0)
        self.assertEqual(hc["price_max"], 60000.0)
        hl = by_key[("合力", "CPCD50", 5.0)]
        self.assertEqual(hl["sample_count"], 1)
        self.assertEqual(hl["price_min"], 80000.0)
        self.assertEqual(hl["price_max"], 80000.0)

    def test_only_unmatched_samples_are_grouped(self):
        """已匹配（residual_rate_label 有值）与被剔除（rejected）的样本不进待审队列。"""
        matched = _sample(url="https://x.test/1", matched_original_price=50000.0,
                          residual_rate_label=0.84, match_level="brand_vehicle_tonnage")
        rejected = _sample(url="https://x.test/2", clean_status="rejected",
                           rejection_reason="bulk_lot_no_per_unit")
        unmatched = _sample(url="https://x.test/3")
        groups = group_unmatched([matched, rejected, unmatched])
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["sample_count"], 1)

    def test_price_range_ignores_missing_prices(self):
        samples = [
            _sample(price=None, url="https://x.test/1"),
            _sample(price=50000.0, url="https://x.test/2"),
        ]
        group = group_unmatched(samples)[0]
        self.assertEqual(group["sample_count"], 2)
        self.assertEqual(group["price_min"], 50000.0)
        self.assertEqual(group["price_max"], 50000.0)

    def test_group_columns_are_stable(self):
        """列名固定，供 SQL VIEW 与 CLI 表头共用同一份定义。"""
        self.assertEqual(
            UNMATCHED_GROUP_COLUMNS,
            ("brand", "model", "tonnage", "sample_count", "price_min", "price_max"),
        )

    def test_empty_input_yields_no_groups(self):
        self.assertEqual(group_unmatched([]), [])


if __name__ == "__main__":
    unittest.main()
