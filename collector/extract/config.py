"""从 PG 直读 AI 配置（#2673），与后端 ConfigService.ResolveConfig 同口径。

collector 与 backend 共享 forklift-net，直接查 ai_feature_bindings → ai_configs，
按 isValidFeature 语义只认 market_sample_extract 这一个 feature_key；
api_key 用 crypto.decrypt_api_key 解密（与后端同一个 SECRET_KEY）。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from .crypto import decrypt_api_key

FEATURE_KEY = "market_sample_extract"


@dataclass(frozen=True)
class ModelConfig:
    api_key: str
    base_url: str
    model: str


def resolve_config(conn: Any, feature_key: str, secret_key: str) -> ModelConfig:
    """读取指定功能绑定的唯一有效配置；未绑定或配置失效时抛 RuntimeError。"""
    if feature_key != FEATURE_KEY:
        raise ValueError(f"无效的 feature_key: {feature_key}")
    row = conn.execute(
        "SELECT c.api_key, c.base_url, c.model "
        "FROM ai_configs c "
        "JOIN ai_feature_bindings b ON b.config_id = c.id "
        "WHERE b.feature_key = %s AND c.is_active = TRUE "
        "ORDER BY b.id ASC LIMIT 1",
        (feature_key,),
    ).fetchone()
    if row is None:
        raise RuntimeError(f"功能 {feature_key} 未绑定有效 AI 配置（需生产首次配置）")
    encrypted_key, base_url, model = row
    return ModelConfig(
        api_key=decrypt_api_key(encrypted_key, secret_key),
        base_url=base_url,
        model=model,
    )
