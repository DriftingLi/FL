"""一期拍卖数据管线采集器（spec #2671 / 票 #2672）。

独立 Python 服务，不进 Go 后端；当前只实现 batch 模式：
一份 fixture 公告 → market_transactions 一行 raw 样本。
"""

from .models import RawSample

__all__ = ["RawSample"]
