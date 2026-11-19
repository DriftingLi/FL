"""LLM 抽取子域（#2673）：配置直读 + 版本化 prompt + 抽取判定。
纯判定与本包内的解析/加密都不依赖 PG；落库与队列写入在 store 层。
"""

from .config import ModelConfig, resolve_config
from .decisions import Decision, decide

__all__ = ["ModelConfig", "resolve_config", "Decision", "decide"]
