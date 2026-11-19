"""校准专用匹配器（#2675）。

分层降级 7→4→3 字段、多匹配取中位数（不复用评估路径的取最高价语义），
纯函数 match(sample, candidates) → 新 RawSample；调用方负责从 PG 读候选原价。
"""

from .matcher import MATCH_VERSION, match

__all__ = ["MATCH_VERSION", "match"]
