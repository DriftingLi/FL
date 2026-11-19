"""把一次公告抽取装配成写回样本 + 入队的纯逻辑（#2673）。"""

from __future__ import annotations

from dataclasses import replace
from typing import Any

from ..models import RawSample
from .decisions import decide, low_confidence_reason
from .prompt import PROMPT_VERSION


def apply_extraction(sample: RawSample, extraction: dict[str, Any]) -> tuple[RawSample, list[tuple[str, str | None]]]:
    """把抽取输出写回样本的 raw 阶段字段，并返回应入队的 (reason, detail) 列表。

    原文留底字段（channel / source_url / raw_description）永不改动；
    只填 structured_specs / 价格 / 置信度 / 抽取版本 / 标签资格。
    """
    specs = extraction.get("structured_specs") or {}
    confidence = extraction.get("field_confidence") or {}
    raw_price = extraction.get("raw_price")
    decision = decide(
        field_confidence=confidence,
        raw_price=raw_price,
    )

    updated = replace(
        sample,
        structured_specs=specs,
        field_confidence=confidence,
        raw_price=raw_price,
        raw_price_kind=extraction.get("raw_price_kind"),
        appraisal_price=extraction.get("appraisal_price"),
        extraction_version=PROMPT_VERSION,
        label_eligible=decision.label_eligible,
    )

    review: list[tuple[str, str | None]] = []
    if decision.label_eligible is False:
        review.append(("price_extraction_failed", None))
    detail = low_confidence_reason(confidence)
    if detail is not None:
        review.append(("low_confidence", detail))
    return updated, review
