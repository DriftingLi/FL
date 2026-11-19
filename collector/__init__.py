"""一期拍卖数据管线采集器（spec #2671 / 票 #2672 / #2673）。

独立 Python 服务，不进 Go 后端；当前只实现 batch 模式：
一份公告 → 直读 AI 配置 → LLM 抽取 → market_transactions 一行 + 抽检入队。
daemon 增量模式是后续票。
"""

from .models import RawSample

__all__ = ["RawSample"]
