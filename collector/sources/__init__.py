"""采集面适配器包（#2679 / #2680）：按 channel 自注册，包内自动发现。

本包是**编排层与平台之间的唯一 seam**。编排层（`collector.cli`）只认
`SourceSpec` 与它的两个纯函数，不认识任何具体平台；平台差异全部收敛在
各适配器模块里。

自动发现的意义是并行开发：新增采集面 = 新增一个模块文件 + 模块内
`register(SourceSpec(...))`，**不需要改本文件、不需要改 CLI、不需要改注册表**。
两张并行开的票因此不会在同一行上冲突（#2680 判据 4）。

适配器模块的纪律：只许 `from .base import ...`，不许 `import collector.sources`
（本包此时正在初始化，反向依赖会拿到半初始化的注册表）。
"""

from __future__ import annotations

import pkgutil
from importlib import import_module

from .base import (
    DetailFetcher,
    Fetch,
    SourceSpec,
    all_sources,
    html_detail,
    register,
    resolve_source,
    source_for_channel,
)

__all__ = [
    "DetailFetcher",
    "Fetch",
    "SourceSpec",
    "all_sources",
    "html_detail",
    "register",
    "resolve_source",
    "source_for_channel",
]

# 非适配器模块：契约本身与包初始化，不参与发现。
_NON_ADAPTER_MODULES = frozenset({"base"})


def _discover() -> None:
    """import 本包下所有适配器模块，让它们各自 register 一次。

    模块名排序后 import：错误信息与日志顺序稳定，不依赖文件系统顺序。
    """
    for info in sorted(pkgutil.iter_modules(__path__), key=lambda m: m.name):
        if info.name in _NON_ADAPTER_MODULES or info.name.startswith("_"):
            continue
        import_module(f"{__name__}.{info.name}")


_discover()
