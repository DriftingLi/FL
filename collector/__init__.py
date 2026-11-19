"""一期拍卖数据管线采集器（spec #2671 / 票 #2672 / #2673 / #2674）。

独立 Python 服务，不进 Go 后端；当前两份命令：
  batch   —— 一份公告 → 直读 AI 配置 → LLM 抽取 → 版本化清洗 → 入库 + 抽检入队；
  reclean —— 读回已入库行 → 按当前清洗规则重算（不重爬公告）。
daemon 增量模式与分层匹配 / 残值率标签是后续票。
"""

from .models import RawSample

__all__ = ["RawSample"]
