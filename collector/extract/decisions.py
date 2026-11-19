"""抽取结果的纯判定（#2673）。

阈值与关键字段集由用户在会话中确认，落为具名常量并写进 ADR-0097：
低置信阈值 0.75；任一关键字段置信度缺失或 < 阈值 ⇒ 进抽检队列；
价格抽取失败 ⇒ 无标签资格（且同时进抽检队列）。
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any, Mapping

# 关键字段：规格字段 + 价格。价格成败单独决定标签资格，但它也是抽检关注对象。
KEY_FIELDS = (
    "brand",
    "model",
    "series",
    "tonnage",
    "year",
    "engine_hours",
    "raw_price",
)
LOW_CONFIDENCE_THRESHOLD = 0.75


@dataclass(frozen=True)
class Decision:
    needs_review: bool
    label_eligible: bool


def _low_confidence_fields(field_confidence: Mapping[str, Any]) -> list[str]:
    """关键字段中置信度缺失或低于阈值的字段名（needs_review 与入队明细共用一份）。"""
    return [
        key
        for key in KEY_FIELDS
        if _confidence_of(field_confidence, key) < LOW_CONFIDENCE_THRESHOLD
    ]


def decide(
    *,
    field_confidence: Mapping[str, Any],
    raw_price: float | None,
) -> Decision:
    """由抽取输出推导抽检与标签资格两个标记。

    低置信判定覆盖规格字段与价格：任一关键字段的置信度缺失或低于阈值即入队。
    标签资格只取决于价格是否成功抽出（raw_price 非空且置信度达标）——
    没有价格就算不出残值率，不管规格抽得多好都不能进标签集。
    """
    needs_review = bool(_low_confidence_fields(field_confidence))

    price_confidence = _confidence_of(field_confidence, "raw_price")
    label_eligible = raw_price is not None and price_confidence >= LOW_CONFIDENCE_THRESHOLD
    return Decision(needs_review=needs_review, label_eligible=label_eligible)


def low_confidence_reason(
    field_confidence: Mapping[str, Any],
) -> str | None:
    """返回抽检明细（低置信字段名列表的 JSON 串），无低置信字段时返回 None。"""
    fields = _low_confidence_fields(field_confidence)
    if not fields:
        return None
    return json.dumps({"low_confidence_fields": fields}, ensure_ascii=False)


def _confidence_of(field_confidence: Mapping[str, Any], key: str) -> float:
    value = field_confidence.get(key)
    if value is None:
        # 关键字段缺置信度 = 不可信，从严处理。
        return 0.0
    try:
        return float(value)
    except (TypeError, ValueError):
        return 0.0
