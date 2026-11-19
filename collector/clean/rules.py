"""清洗阶段单一版本化规则模块（#2674）。

第一性原理：采集层只 fetch + 原始留底，清洗是独立的纯函数阶段——
吃一条已抽取的 RawSample（+ 可选的已入库样本列表用于去重），产出清洗后的
样本与需入抽检队列的 (reason, detail) 列表。不碰网络、不碰 PG、不改入参，
因此「重跑清洗不用重爬」：改了规则版本就用同一批留底行重算即可。

七条硬规则（wayfinder #2666「标签口径」票裁定，顺序即优先级）：
  ① 批量包价剔除（无逐台字段）
  ② 报废注销 / 无电池车剔除（废铁价 ≠ 使用价值）
  ③ 占位价量级过滤（吨位-价格合理性区间）
  ④ 面议丢弃
  ⑤ 区间报价丢弃（单值字段是幌子）
  ⑥ 车商批量帖去重（图哈希 / 描述相似度 / 同公司跨城）
  ⑦ 溢价 > 评估价 × 1.5 标记降权（不静默剔除）
另：买受人身份剥离（spec #2671 PIPL 口径）——结构化字段与原文留底都不含身份。
清洗得率不设硬门槛（#2662 已裁），本模块只如实产出状态，不设通过率门。
"""

from __future__ import annotations

import re
from dataclasses import replace
from difflib import SequenceMatcher
from typing import Any, Iterable

CLEAN_VERSION = "v1"

# rule reason 常量（与迁移 000064 reason 列取值域一致，便于聚合）。
REASON_BULK_LOT = "bulk_lot_no_per_unit"
REASON_SCRAPPED = "scrapped_or_no_battery"
REASON_PLACEHOLDER = "placeholder_price"
REASON_NEGOTIABLE = "negotiable_price"
REASON_RANGE = "range_price"
REASON_DUP_IMAGE = "duplicate_image_hash"
REASON_DUP_DESC = "duplicate_description_similarity"
REASON_DUP_SELLER = "duplicate_same_company_cross_city"
REASON_PREMIUM = "premium_anomaly"
REASON_IDENTITY_STRIPPED = "identity_stripped"

# 清洗阶段产生的全部 review reason；重跑清洗时据此清掉旧行再重入，保证幂等。
CLEAN_REVIEW_REASONS = (
    REASON_BULK_LOT, REASON_SCRAPPED, REASON_PLACEHOLDER, REASON_NEGOTIABLE,
    REASON_RANGE, REASON_DUP_IMAGE, REASON_DUP_DESC, REASON_DUP_SELLER,
    REASON_PREMIUM, REASON_IDENTITY_STRIPPED,
)

PREMIUM_MULTIPLIER = 1.5
# 占位价量级下限：price < 吨位 × 此值 → 明显不合理（如「7 吨挂 8 元」）。
MIN_PRICE_PER_TONNE = 1000.0
DESC_SIMILARITY_THRESHOLD = 0.92

# 买受人身份字段：从 structured_specs 与 raw_description 两处剥离。
_IDENTITY_FIELDS = ("buyer_name", "bidder_no", "id_no", "buyer_id", "竞买人", "买受人")
# 原文 body 里的行内身份：即使结构化字段没抽到，带标签的值也要脱敏（PIPL 留底亦不含身份）。
# 保留标签与分隔（属「原文其它内容」），只替换紧随其后的身份值 token。
_IDENTITY_TEXT_PATTERNS = (
    re.compile(
        r"(竞买人编号|买受人|竞买人|竞买号|身份证号?|证件号?)"
        r"(\s*[:：]?\s*)([^\s，,。（）()]+)"
    ),
)
_REDACTED = "[已脱敏]"


def _specs(sample) -> dict[str, Any]:
    return sample.structured_specs or {}


def _truthy(value) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        return value.strip().lower() in ("true", "yes", "1", "是")
    return bool(value)


def _price_text(sample) -> str:
    return str(_specs(sample).get("price_text", "") or "")


def _reject(sample, reason: str) -> tuple:
    return (
        replace(sample, clean_status="rejected", rejection_reason=reason, label_eligible=False),
        [(reason, None)],
    )


def _redact_text(value: str) -> str:
    """把原文里带标签的行内身份值换成占位符；标签与无身份正文原样保留。"""
    for pattern in _IDENTITY_TEXT_PATTERNS:
        value = pattern.sub(lambda m: m.group(1) + m.group(2) + _REDACTED, value)
    return value


def _strip_identity(sample):
    """返回剥离身份后的 (sample, 是否剥离了字段)。原文非身份内容保留。"""
    specs = dict(_specs(sample))
    raw = dict(sample.raw_description or {})
    stripped = False
    for key in _IDENTITY_FIELDS:
        if key in specs:
            specs.pop(key)
            stripped = True
        if key in raw:
            raw.pop(key)
            stripped = True
    # 行内身份：即使没抽成字段，正文里「买受人 张三 / 竞买号 A1234」也不得留底（PIPL）。
    for key, value in list(raw.items()):
        if isinstance(value, str):
            redacted = _redact_text(value)
            if redacted != value:
                raw[key] = redacted
                stripped = True
    updated = replace(sample, structured_specs=specs, raw_description=raw)
    return updated, stripped


def _rule_bulk_lot(sample):
    specs = _specs(sample)
    if _truthy(specs.get("bulk_lot")) or (
        specs.get("lot_count") is not None and (specs.get("tonnage") is None and specs.get("model") is None)
    ):
        return _reject(sample, REASON_BULK_LOT)
    return None


def _rule_scrapped(sample):
    specs = _specs(sample)
    if _truthy(specs.get("scrapped")) or specs.get("has_battery") is False:
        return _reject(sample, REASON_SCRAPPED)
    return None


def _rule_placeholder(sample):
    specs = _specs(sample)
    price = sample.raw_price
    tonnage = specs.get("tonnage")
    if price is not None and tonnage:
        try:
            if float(price) < float(tonnage) * MIN_PRICE_PER_TONNE:
                return _reject(sample, REASON_PLACEHOLDER)
        except (TypeError, ValueError):
            pass
    return None


def _rule_negotiable(sample):
    text = _price_text(sample)
    if "面议" in text or "另询" in text or "价格另议" in text:
        return _reject(sample, REASON_NEGOTIABLE)
    return None


def _rule_range(sample):
    specs = _specs(sample)
    text = _price_text(sample)
    if specs.get("price_range") or ("-" in text and "万" in text):
        return _reject(sample, REASON_RANGE)
    return None


def _dedup_hit(sample, existing: Iterable) -> str | None:
    """车商批量帖去重三形态：图哈希 / 描述相似度 / 同公司跨城。"""
    specs = _specs(sample)
    body = str((sample.raw_description or {}).get("body", "") or "")
    image_hashes = set(specs.get("image_hashes") or [])
    seller = specs.get("seller")
    city = specs.get("city")

    for prior in existing:
        # 去重只跟「别的帖子」比；同来源 URL 是同一条，不能自命中。
        if prior.source_url == sample.source_url:
            continue
        pspecs = _specs(prior)
        p_hashes = set(pspecs.get("image_hashes") or [])
        if image_hashes and image_hashes & p_hashes:
            return REASON_DUP_IMAGE
        p_body = str((prior.raw_description or {}).get("body", "") or "")
        if body and p_body and SequenceMatcher(None, body, p_body).ratio() >= DESC_SIMILARITY_THRESHOLD:
            return REASON_DUP_DESC
        if seller and pspecs.get("seller") == seller and city and pspecs.get("city") != city:
            return REASON_DUP_SELLER
    return None


def _rule_premium(sample) -> tuple | None:
    """溢价 > 评估价 × 1.5：标记降权（cleaned + premium_anomaly + 入队），不剔除。"""
    price = sample.raw_price
    appraisal = sample.appraisal_price
    if price is not None and appraisal:
        try:
            if float(price) > float(appraisal) * PREMIUM_MULTIPLIER:
                updated = replace(sample, clean_status="cleaned", premium_anomaly=True)
                return updated, [(REASON_PREMIUM, None)]
        except (TypeError, ValueError):
            pass
    return None


def _apply_rules(sample, existing: Iterable = ()) -> tuple:
    """按优先级跑完身份剥离 + 七规则，返回 (样本, review)。不改入参。"""
    work, stripped = _strip_identity(sample)
    review: list[tuple[str, str | None]] = [(REASON_IDENTITY_STRIPPED, None)] if stripped else []

    for rule in (_rule_bulk_lot, _rule_scrapped, _rule_placeholder, _rule_negotiable, _rule_range):
        outcome = rule(work)
        if outcome is not None:
            # 规则返回的样本已从 work（含身份剥离结果）派生，无需再补字段。
            cleaned_sample, rule_review = outcome
            return cleaned_sample, review + rule_review

    dup = _dedup_hit(work, existing)
    if dup is not None:
        # work 已是身份剥离后的样本；_reject 只改清洗列，其余字段（含剥离结果）保留。
        rejected_sample, _ = _reject(work, dup)
        return rejected_sample, review + [(dup, None)]

    if work.raw_price is not None and work.field_confidence:
        work = replace(work, clean_status="cleaned")

    premium = _rule_premium(work)
    if premium is not None:
        work, premium_review = premium
        review = review + premium_review

    return work, review


def clean(sample, existing: Iterable = ()) -> tuple:
    """清洗一条已抽取样本，返回 (清洗后样本, review 列表)。纯函数，不改入参。

    顺序：身份剥离 → 剔除规则①②③④⑤ → 去重⑥ → 溢价标记⑦。所有结果统一盖上
    CLEAN_VERSION 落库，使「哪版规则判的」可审计、两代规则可分离重跑。
    剔除类规则命中即返回（reason 入 rejection_reason）；溢价是标记而非剔除。
    """
    cleaned, review = _apply_rules(sample, existing)
    return replace(cleaned, clean_version=CLEAN_VERSION), review
