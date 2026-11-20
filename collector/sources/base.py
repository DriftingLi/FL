"""采集面适配器契约与注册表（#2679 / #2680 的共享 seam）。

第一性原理：spec #2671 要求「历史回补与增量采集是同一组件的两种模式」、
「清洗与匹配逻辑共用一份、不漂移」。多平台采集面（rmfysszc / 京东 / 公拍网 /
产权交易所省镜像）之间**唯一的差异**是两个纯函数：

  ① 列表页 HTML → 发现条目（`DiscoveredAnnouncement`）；
  ② 详情页（或详情接口）→ raw 样本（`RawSample`，含价格与逐字段置信度）。

把这两件事抽成按 `channel` 注册的 `SourceSpec` 之后，编排层（年份过滤 → 幂等判重 →
抽取/直取 → 清洗 → 匹配 → 落库 → 得率报告）对平台完全无感知：加一个采集面 =
加一个模块，不改编排层、不改 CLI、不改本文件的注册表。

**注册靠模块自注册 + 包内自动发现**（`collector/sources/__init__.py` 扫本包下的
模块并 import，模块内调 `register()`）。所以两张并行开发的票各自只碰自己的文件，
不会在同一个 `__init__` 里抢同一行（#2680 判据 4「与京东/gpai 适配器无共享状态
冲突」）。适配器模块只许 `from .base import ...`，不许 import 本包（半初始化）。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Callable

from ..backfill import DiscoveredAnnouncement
from ..models import RawSample

# 抓取函数：url → 文本（HTML 或 JSONP 响应体）。生产是 CrawlClient.fetch，
# 测试注入录制 fixture，适配器自己不碰网络（不 import urllib）。
Fetch = Callable[[str], str]

# 列表解析：(列表页 HTML, 列表页 URL) → 发现条目。
#
# 第二个参数不是装饰：列表页里的详情链接普遍是相对地址（`item2.do?...`、
# `jyjgDetail?guid=...`），解析器需要列表页 URL 才能拼出绝对地址；而绝对地址
# 正是幂等判重的键（`market_transactions.source_url`）。少了它，适配器只能猜 base。
ParseListing = Callable[[str, str], list[DiscoveredAnnouncement]]

# 详情获取：url + 抓取函数 → 0..n 条 raw 样本。允许一个详情页产出多条样本
# （如「批量处置」公告里逐台标的），也允许适配器直接打结构化接口而不抓页面。
DetailFetcher = Callable[[str, Fetch], list[RawSample]]


@dataclass(frozen=True)
class SourceSpec:
    """一个采集面的全部平台差异（除两个纯函数外，只剩识别与合规信息）。"""

    channel: str
    # robots.txt 绑定用的站点根（含 scheme + host，不带路径）。
    base_url: str
    # 本采集面会访问的全部 origin（`scheme://host`，无路径）。采集层按 origin 各自
    # 读 robots、各自 fail-closed——不能拿 paimai.jd.com 的 robots 去放行
    # api.m.jd.com（RFC 9309 按 host 生效）。
    origins: tuple[str, ...]
    # 用 URL 片段识别采集面（如 "paimai.jd.com/"、"gpai.net/sf/"）。
    url_markers: tuple[str, ...]
    parse_listing: ParseListing
    fetch_detail: DetailFetcher
    # True ⇒ 字段来自结构化数据（JSONP / 服务端渲染表格），跳过 LLM 抽取；字段
    # 置信度由适配器按「取值机制」直接给（JSONP 与表格直取 = 1.0，标题正则 = 0.9）。
    # False ⇒ 非结构化正文，走既有 LLM 抽取（rmfysszc 的路径）。
    structured: bool = False
    # True ⇒ 无发布日期的发现条目也保留。回补（rmfysszc）靠日期卡年份窗口，
    # 无日期无法归年必须丢；结构化平台是「在拍/已结束标的」，本身就是当前窗口。
    keep_undated: bool = False
    # 默认发现入口：给了之后 CLI 不传 --list-url 也能跑（结构化平台的频道列表页）。
    default_list_urls: tuple[str, ...] = ()

    def matches(self, url: str) -> bool:
        """URL 是否属于本采集面。片段匹配足够——域名与频道路径都是稳定前缀。"""
        return any(marker in url for marker in self.url_markers)


_REGISTRY: dict[str, SourceSpec] = {}


def register(spec: SourceSpec) -> SourceSpec:
    """登记一个采集面（适配器模块在 import 时调用一次）。"""
    if not spec.channel:
        raise ValueError("SourceSpec.channel 不能为空")
    existing = _REGISTRY.get(spec.channel)
    if existing is not None:
        raise ValueError(
            f"采集面 {spec.channel!r} 重复登记（已存在 {existing.base_url}）——"
            "channel 是入库列 market_transactions.channel 的取值，不能一值两源"
        )
    if not spec.url_markers:
        raise ValueError(f"采集面 {spec.channel!r} 必须给 url_markers，否则无法按 URL 识别")
    if not spec.origins:
        raise ValueError(f"采集面 {spec.channel!r} 必须给 origins，否则无法读 robots")
    for origin in spec.origins:
        if not origin.startswith(("http://", "https://")):
            raise ValueError(
                f"采集面 {spec.channel!r} 的 origins 必须是 scheme://host 形式，收到 {origin!r}"
            )
    _REGISTRY[spec.channel] = spec
    return spec


def all_sources() -> list[SourceSpec]:
    """已登记的采集面（按 channel 排序，供 CLI 列表与测试遍历）。"""
    return [_REGISTRY[name] for name in sorted(_REGISTRY)]


def source_for_channel(channel: str) -> SourceSpec:
    """按 channel 取采集面；未知 channel 直接报错（不猜、不回落默认）。"""
    spec = _REGISTRY.get(channel)
    if spec is None:
        known = ", ".join(sorted(_REGISTRY)) or "（无）"
        raise ValueError(f"未知采集面 {channel!r}；已登记：{known}")
    return spec


def resolve_source(url: str) -> SourceSpec:
    """按 URL 识别采集面。多命中取「最长片段」那个——更具体的频道路径优先于域名。

    识别不出时报错并列出已知采集面：静默回落到某个默认源会把别的平台的 HTML
    当成 rmfysszc 公告解析，产出一批看似正常、实为垃圾的 raw 行。
    """
    best: tuple[int, SourceSpec] | None = None
    for spec in _REGISTRY.values():
        for marker in spec.url_markers:
            if marker and marker in url:
                if best is None or len(marker) > best[0]:
                    best = (len(marker), spec)
    if best is None:
        known = "，".join(
            f"{s.channel}（{' / '.join(s.url_markers)}）" for s in all_sources()
        ) or "（无）"
        raise ValueError(f"无法识别 URL 所属采集面：{url}；已登记：{known}")
    return best[1]


def html_detail(parse: Callable[[str, str], list[RawSample]]) -> DetailFetcher:
    """把「(html, url) → 样本」的纯解析函数适配成 DetailFetcher（抓一次详情页）。

    绝大多数采集面（公拍网 / 产权交易所 / rmfysszc）都是「抓详情页 → 解析」，
    只有需要打 JSONP 接口的平台（京东）才自定义 fetch_detail。
    """

    def fetch_detail(url: str, fetch: Fetch) -> list[RawSample]:
        return parse(fetch(url), url)

    return fetch_detail
