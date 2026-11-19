"""版本化抽取 prompt（#2673）。

prompt 版本即写回样本的 extraction_version：两者同源，避免「代码用了新 prompt、
样本标着旧版本」的漂移。测试只校验版本被带进消息与响应可解析，不测 prompt 质量。
"""

from __future__ import annotations

import json
from typing import Any

PROMPT_VERSION = "v1"

SYSTEM_PROMPT = f"""你是叉车拍卖公告的结构化抽取器（prompt 版本 {PROMPT_VERSION}）。
从公告正文中抽取以下字段，只输出 JSON，不要输出解释：
- structured_specs: brand / model / series / tonnage / year / engine_hours
- field_confidence: 上述每个字段的置信度（0-1 数字），外加 raw_price
- raw_price: 挂牌价或成交价的数字（认不出来填 null）
- raw_price_kind: listing / deal / null
- appraisal_price: 评估价或起拍价的数字（认不出来填 null）
字段认不出来时对应 confidence 给低值，不要编造。"""


def build_messages(body: str) -> list[dict[str, str]]:
    return [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": body},
    ]


def parse_response(text: str) -> dict[str, Any]:
    """解析模型 JSON 输出；非 JSON 或非对象抛 ValueError（fail-closed，不入库脏数据）。"""
    try:
        data = json.loads(text)
    except (json.JSONDecodeError, TypeError) as exc:
        raise ValueError(f"抽取输出不是合法 JSON: {exc}") from exc
    if not isinstance(data, dict):
        raise ValueError("抽取输出不是 JSON 对象")
    return data
