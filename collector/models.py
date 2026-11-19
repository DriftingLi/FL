"""collector 领域数据结构。

本票（#2672）只落 raw 样本：渠道 / 来源 URL / 原始描述 jsonb 三项可断言；
抽取 / 清洗 / 匹配尚未接入，全部置默认值。字段与迁移 000063 一一对应，
后续票把各阶段结果映射到同一行的其余列，不需改表。
"""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
from datetime import datetime
from typing import Any

# 采集渠道稳定标识（与 CONTEXT.md 新词「市场真值」同源）。
CHANNEL_RMFYSSZC = "rmfysszc"


@dataclass(frozen=True)
class RawSample:
    channel: str
    source_url: str
    raw_description: dict[str, Any]
    # 原始价格：挂牌价 / 成交价（raw 阶段尚未抽取，留空）。
    raw_price: float | None = None
    raw_price_kind: str | None = None
    # 评估价 / 起拍价、成交日期、结构化规格：后续抽取票填。
    appraisal_price: float | None = None
    auction_date: datetime | None = None
    structured_specs: dict[str, Any] = field(default_factory=dict)
    # 清洗 / 抽取 / 匹配 / 标签：后续票填，raw 阶段即默认值。
    clean_status: str = "raw"
    rejection_reason: str | None = None
    field_confidence: dict[str, Any] = field(default_factory=dict)
    extraction_version: str | None = None
    match_level: str | None = None
    matched_original_range: dict[str, Any] = field(default_factory=dict)
    premium_anomaly: bool = False
    residual_rate_label: float | None = None
    matched_original_price: float | None = None

    def to_row(self) -> dict[str, Any]:
        """转成迁移 000063 的列名 → 值（jsonb 列由 store 层处理自适应）。"""
        row = asdict(self)
        row["auction_date"] = (
            self.auction_date.isoformat() if self.auction_date else None
        )
        return row
