"""校准专用分层匹配器（#2675）——纯判定、版本化、可分离重跑。

对齐 #2671/ADR-0096 决策：
  - 分层降级：精确 7 字段 → 品牌+车型+系列+吨位 → 品牌+车型+吨位；
  - 多匹配取原价中位数（不是评估路径 ORDER BY original_price DESC LIMIT 1 的取最高价，
    那会对残值率标签系统性压低偏差）；
  - 记录 match_level + matched_original_range（min/max/count/width），区间宽度进可信度；
  - 残值率 = 成交价 ÷ 匹配原价；未匹配保留 label_eligible、标签留空（partial index
    自动排除出校准集）；
  - 不碰评估路径（backend/internal/valuation 零改动）。

纯函数：只读 sample 与 candidates，用 dataclasses.replace 产出新样本，不改入参。
"""

from __future__ import annotations

from dataclasses import replace
from statistics import median
from typing import Any, Sequence

from ..models import RawSample

MATCH_VERSION = "v1"

LEVEL_EXACT7 = "exact7"
LEVEL_BRAND_VEHICLE_SERIES_TONNAGE = "brand_vehicle_series_tonnage"
LEVEL_BRAND_VEHICLE_TONNAGE = "brand_vehicle_tonnage"

# 7 字段精确匹配的键（照 original_prices 的 7 字段，来自 baseline 000001）。
_SEVEN_KEYS = (
    "brand", "vehicle_type", "series", "tonnage",
    "config_type", "mast_type", "mast_height_mm",
)


def _vehicle_type(specs: dict[str, Any]) -> Any:
    """车型信号：抽取侧产出 model，原价表列名 vehicle_type，两者等价。

    用 `or` 而非 `dict.get(key, default)`：后者在 key 存在但值为 None 时不会回退到 model。
    """
    value = specs.get("vehicle_type") or specs.get("model")
    return value if value not in (None, "") else None


def _norm(value: Any) -> Any:
    return value.strip() if isinstance(value, str) else value


def _field_equal(required: Any, actual: Any) -> bool:
    """匹配字段值相等：数值按 float 比（PG Decimal ↔ Python float），其余按归一化字符串比。"""
    if isinstance(required, bool) or isinstance(actual, bool):
        return required is actual
    if isinstance(required, (int, float)):
        try:
            return float(actual) == float(required)
        except (TypeError, ValueError):
            return False
    return _norm(actual) == _norm(required)


def _match_at(candidates: Sequence[dict[str, Any]], requirement: dict[str, Any]) -> list[float]:
    """只保留「每个要求字段的值都与候选相等」的原价——绝不宽松命中。

    requirement 是字段名→样本值；逐字段比对候选值（不是只查键存在）。
    这是校准匹配与评估路径取最高价语义的关键区别：只取真正同规格的原价。
    """
    matched: list[float] = []
    for row in candidates:
        if row.get("original_price") is None:
            continue
        if all(
            key in row and _field_equal(value, row[key])
            for key, value in requirement.items()
        ):
            matched.append(float(row["original_price"]))
    return matched


def _confidence(width: float, mid: float) -> float:
    """区间相对越宽可信度越低；单匹配 width=0 ⇒ 1.0，mid<=0 ⇒ 0.0（异常原价不置信）。"""
    if mid <= 0:
        return 0.0
    return 1.0 / (1.0 + width / mid)


def match(sample: RawSample, candidates: Sequence[dict[str, Any]]) -> RawSample:
    """把样本与原价候选按 7→4→3 分层匹配，写回匹配/标签列，返回新样本。

    candidates 元素为 dict，至少含 original_price；键名同 original_prices 列名。
    纯函数：不改 sample 与 candidates。
    """
    specs = sample.structured_specs or {}
    brand = _norm(specs.get("brand"))
    vehicle = _norm(_vehicle_type(specs))
    series = _norm(specs.get("series"))
    tonnage = specs.get("tonnage")

    # 按「可用键是否齐全」决定哪一层：先试 7 字段，再 4，再 3。
    layered: list[tuple[str, dict[str, Any]]] = []
    seven = {k: _norm(specs.get(k)) for k in _SEVEN_KEYS}
    seven["vehicle_type"] = vehicle
    if all(seven[k] is not None for k in _SEVEN_KEYS):
        layered.append((LEVEL_EXACT7, seven))
    if None not in (brand, vehicle, series, tonnage):
        layered.append((LEVEL_BRAND_VEHICLE_SERIES_TONNAGE, {
            "brand": brand, "vehicle_type": vehicle, "series": series, "tonnage": tonnage,
        }))
    if None not in (brand, vehicle, tonnage):
        layered.append((LEVEL_BRAND_VEHICLE_TONNAGE, {
            "brand": brand, "vehicle_type": vehicle, "tonnage": tonnage,
        }))

    for level, requirement in layered:
        prices = _match_at(candidates, requirement)
        if prices:
            return _apply_match(sample, level, prices)

    # 未匹配：留底、无标签、不进校准集；label_eligible 语义（价格抽取成功）不动。
    return replace(
        sample,
        match_level=None,
        matched_original_price=None,
        matched_original_range={},
        residual_rate_label=None,
        match_version=MATCH_VERSION,
    )


def _apply_match(sample: RawSample, level: str, prices: list[float]) -> RawSample:
    mid = float(median(prices))
    lo, hi = min(prices), max(prices)
    width = hi - lo
    rng = {"min": lo, "max": hi, "count": len(prices), "width": width}
    confidence = _confidence(width, mid)
    field_confidence = dict(sample.field_confidence or {})
    field_confidence["matched_original_price"] = confidence
    # PG 读回的 raw_price 是 Decimal，Python float 与之不能相除——统一转 float。
    raw = float(sample.raw_price) if sample.raw_price is not None else None
    residual = raw / mid if raw is not None and mid > 0 else None
    return replace(
        sample,
        match_level=level,
        matched_original_price=mid,
        matched_original_range=rng,
        residual_rate_label=residual,
        field_confidence=field_confidence,
        match_version=MATCH_VERSION,
    )
