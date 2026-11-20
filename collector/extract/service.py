"""把一次公告抽取装配成写回样本 + 入队的纯逻辑（#2673）。"""

from __future__ import annotations

from dataclasses import replace
from typing import Any

from ..models import RawSample
from .decisions import (
    claimed_low_confidence_reason,
    decide,
    low_confidence_reason,
)
from .prompt import PROMPT_VERSION

# 结构化直取样本的 extraction_version：与 LLM 抽取的 PROMPT_VERSION 分开，
# 让「这个字段是哪条路径产出的」在库里可分辨（重跑/审计时不能混为一谈）。
STRUCTURED_VERSION = "structured-v1"


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


def apply_structured(sample: RawSample) -> tuple[RawSample, list[tuple[str, str | None]]]:
    """结构化直取样本的装配（#2679 / #2680）：跳过 LLM，只做判定与盖版本。

    入参已由适配器填好 structured_specs / field_confidence / 价格三件套（字段来自
    JSONP 响应或服务端渲染表格，是来源事实而非模型猜测）。本函数只做两件事：
      ① 用与 LLM 路径**同一套** `decide` 判标签资格——价格门槛不能因采集面而放宽，
         否则结构化样本会绕过「无价格不算标签」这条硬约束；
      ② 盖 extraction_version 为 STRUCTURED_VERSION，让入库行能区分来源路径。

    抽检口径按 `claimed_low_confidence_reason`：只审声称取到但置信不足的字段，
    不把「源站不提供的字段」当抽取失败（详见该函数说明）。
    """
    decision = decide(
        field_confidence=sample.field_confidence or {},
        raw_price=sample.raw_price,
    )
    updated = replace(
        sample,
        extraction_version=STRUCTURED_VERSION,
        label_eligible=decision.label_eligible,
    )
    review: list[tuple[str, str | None]] = []
    if decision.label_eligible is False:
        review.append(("price_extraction_failed", None))
    detail = claimed_low_confidence_reason(sample.field_confidence or {})
    if detail is not None:
        review.append(("low_confidence", detail))
    return updated, review
