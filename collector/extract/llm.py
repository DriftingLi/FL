"""OpenAI 兼容 chat 调用（#2673），仅用标准库 urllib。

真实生产链路 = config 直读 + 本函数调用；契约测试注入 extractor stub，
不打网络、不测 prompt 质量（spec #2671 测试口径）。
"""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.request
from typing import Callable

from .config import ModelConfig
from .prompt import build_messages, parse_response

Extractor = Callable[[list[dict[str, str]], ModelConfig], str]


def call_llm(messages: list[dict[str, str]], config: ModelConfig) -> str:
    url = config.base_url.rstrip("/") + "/chat/completions"
    body = json.dumps(
        {"model": config.model, "messages": messages, "temperature": 0},
        ensure_ascii=False,
    ).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=body,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {config.api_key}",
        },
        method="POST",
    )
    timeout = float(os.environ.get("COLLECTOR_LLM_TIMEOUT", "30"))
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            payload = json.loads(resp.read().decode("utf-8"))
    except urllib.error.URLError as exc:
        raise RuntimeError(f"LLM 调用失败: {exc}") from exc
    try:
        return payload["choices"][0]["message"]["content"]
    except (KeyError, IndexError, TypeError) as exc:
        raise RuntimeError(f"LLM 响应缺少 choices[0].message.content: {payload}") from exc


def extract(
    body: str,
    config: ModelConfig,
    extractor: Extractor | None = None,
) -> dict:
    """一次抽取 = 版本化 prompt + （真实或桩）模型调用 + JSON 解析。"""
    messages = build_messages(body)
    text = extractor(messages, config) if extractor is not None else call_llm(messages, config)
    return parse_response(text)
