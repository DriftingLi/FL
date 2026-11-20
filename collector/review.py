"""未匹配待审队列的聚合判定（#2676）——纯函数、无 IO。

对齐 #2671「原价反哺」决策：**不自动写回原价表**；未匹配样本按
品牌×型号×吨位聚合成待审队列（一期为聚合视图，无管理端 UI），
人工定期审核补原价条目，补入后重跑匹配回收标签。

本模块只做「把已入库样本按待审口径聚成组」的纯判定；SQL VIEW（迁移 000067）
是同一口径的落库形态，二者列名共用 `UNMATCHED_GROUP_COLUMNS`，避免两份真相。
"""

from __future__ import annotations

from typing import Any, Iterable

from .models import RawSample

# 聚合列：品牌 / 型号 / 吨位 + 样本数 + 成交价区间。SQL VIEW 与 CLI 表头共用。
UNMATCHED_GROUP_COLUMNS = (
    "brand", "model", "tonnage", "sample_count", "price_min", "price_max",
)

# 待审口径：未匹配（无标签）且未被清洗剔除。raw 行也进待审——它只是还没跑完管线。
_REJECTED = "rejected"


def unmatched_group_key(sample: RawSample) -> tuple[Any, Any, Any]:
    """待审聚合键 = (品牌, 型号, 吨位)；缺字段时为 None（仍可成组，不丢样本）。"""
    specs = sample.structured_specs or {}
    return (
        specs.get("brand"),
        specs.get("model"),
        specs.get("tonnage"),
    )


def _is_pending(sample: RawSample) -> bool:
    """待审样本 = 有价格资格但还没拿到标签，且没被清洗剔除。

    已匹配（residual_rate_label 有值）的样本不需要人工补原价；
    被剔除（rejected）的样本本身不该进校准集，补原价也无意义。
    """
    if sample.clean_status == _REJECTED:
        return False
    return sample.residual_rate_label is None


def group_unmatched(samples: Iterable[RawSample]) -> list[dict[str, Any]]:
    """把未匹配样本按品牌×型号×吨位聚合成待审组（含样本数与成交价区间）。

    纯函数：不改入参，返回按样本数降序的组列表。价格缺失的样本计入样本数
    但不参与区间计算（区间取有价样本的 min/max）。
    """
    buckets: dict[tuple[Any, Any, Any], list[RawSample]] = {}
    for sample in samples:
        if not _is_pending(sample):
            continue
        buckets.setdefault(unmatched_group_key(sample), []).append(sample)

    groups: list[dict[str, Any]] = []
    for (brand, model, tonnage), members in buckets.items():
        prices = [
            float(s.raw_price) for s in members if s.raw_price is not None
        ]
        groups.append({
            "brand": brand,
            "model": model,
            "tonnage": tonnage,
            "sample_count": len(members),
            "price_min": min(prices) if prices else None,
            "price_max": max(prices) if prices else None,
        })
    # 样本多的组优先处理；键做次级排序保证确定性输出。
    groups.sort(key=lambda g: (-g["sample_count"], str(g["brand"]), str(g["model"]), str(g["tonnage"])))
    return groups
