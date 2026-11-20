"""rmfysszc 采集面适配器（#2672–#2678 的既有渠道，收进 seam 由 #2679 统一）。

本模块不新增解析逻辑：列表页解析复用 `backfill.parse_announcement_list`、详情页
解析复用 `parsers.rmfysszc.parse_announcement`，只是把它们登记成一个 `SourceSpec`，
让编排层不再硬编码「rmfysszc 就是唯一源」。

它是唯一 `structured=False` 的采集面——正文是非结构化公告，必须走 LLM 抽取。
其它采集面（京东 JSONP / 公拍网表格 / 产权交易所表格）都是结构化直取。
"""

from __future__ import annotations

from ..backfill import BASE, parse_announcement_list
from ..models import CHANNEL_RMFYSSZC
from ..parsers.rmfysszc import parse_announcement
from .base import SourceSpec, html_detail, register

SPEC = register(
    SourceSpec(
        channel=CHANNEL_RMFYSSZC,
        base_url=BASE,
        # 回补列表页实测落在 www1.rmfysszc.gov.cn（与详情页 www 不同 host），
        # 两个 host 都要各自核验 robots——这正是多 origin 采集面的起因。
        origins=(BASE, "https://www1.rmfysszc.gov.cn"),
        # 列表页 URL 形如 /Agency/pmgg.aspx?fid=100，详情页含 rm_xmdetail / rm_gzgsdetail。
        url_markers=("rmfysszc.gov.cn/Agency/", "rm_xmdetail", "rm_gzgsdetail"),
        parse_listing=lambda html, _list_url: parse_announcement_list(html),
        fetch_detail=html_detail(lambda html, url: parse_announcement(html, url)),
        structured=False,
        # 回补靠发布日期卡 2015–2026 窗口，无日期无法归年 ⇒ 必须丢（ADR-0101 决策七）。
        keep_undated=False,
    )
)
