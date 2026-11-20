"""公拍网（gpai.net）采集面适配器（#2679 判据 2/3）。

## 两个频道，能力不同（实测）

**司法频道 `www.gpai.net/sf/`** —— 服务端渲染，可采：
  - 列表页 `/sf/`（实测 68,491 B，56 个条目）里每条含起拍价与评估价，但**只有在拍标的**
    （状态徽章只有「正在拍卖」/「正在变卖」，0 个「已结束」）。
  - 详情页 `/sf/item2.do?Web_Item_ID=<id>`（在拍 733,595 B / 已结束 99,978 B）含
    `起拍价：<span id="Price_Start">…</span>`、`评估价：…`、成交时另有
    `<div class="d-m-price"><p><span>成交价：</span><b class='price-red'>195899</b>`。
  - robots：`www.gpai.net/robots.txt` 明确 `Disallow:/sf/search.do`、`/sf/noticeDetail.do`、
    `/api/`，但 `/sf/` 与 `/sf/item2.do` **不在禁止列表** ⇒ 在拍标的可合规采集。

**资产频道 `zc.gpai.net`** —— Vue SPA，只有列表接口可用：
  - `/api/item/list?pageNum=N&pageSize=5` 可用，但 pageSize / pageNum / keyword 参数
    **全部被忽略**（page=1/2/10/100/1000/50000 返回同一批 12 条）⇒ 只能取到一页。
  - 详情接口全废：`/api/item/item`、`/api/item/bidRecord` → `{"code":"9999","msg":"info为空"}`；
    `/api/item/realTimeBidData` → `data:null`；`/api/item/agencyPrice` → `未登录`。
    `zc/detail?id=<id>` 是 1,450 B 的 SPA 空壳。
  - ⇒ 资产频道**只能落「在拍标的 + 起拍价」**（`raw_price_kind='listing'`），
    拿不到成交价与评估价。这是平台能力边界，不是实现偷懒。

## 已结束标的的合规冲突（必须在 ADR 里记明）

已成交样本只出现在 `s.gpai.net/sf/search.do?action=court&restate=3`，而该路径：
  - 属 `www.gpai.net/robots.txt` 明示的 `Disallow:/sf/search.do`；
  - 且 `s.gpai.net/robots.txt` 拿不到（302 返回京东首页 HTML，异常镜像）⇒ fail-closed。
⇒ **已成交样本在合规默认下不可采**，必须 `--allow-robots-override`。

## 反爬运行态：验证码页必须当失败

2026-11-20 14:39 起，gpai 对诚实 UA（`ForkliftValuationCollector/1.0 (+contact: …)`）
返回 1,774 B 的滑块页（`seccaptcha.haplat.net`），页面 `infoString` base64 解出
`{"type":"blacklist", …}` —— **我们的 UA 已被拉黑**。

所以适配器必须把验证码页识别成**抓取失败**（抛错，由编排层的毒丸隔离接住），
绝不能解析出空样本：静默返回 `[]` 会让得率报告全绿、库里却一行没有，
这种「看起来成功」的失败比报错难查得多。
"""

from __future__ import annotations

import re
from datetime import datetime, timezone
from typing import Any
from urllib.parse import urljoin

from ..backfill import DiscoveredAnnouncement
from ..models import CHANNEL_GPAI, RawSample
from .base import Fetch, SourceSpec, register

_SF_BASE = "https://www.gpai.net/sf/"
_ZC_BASE = "https://zc.gpai.net/"

# 列表页里的详情链接：实测形如 `<a href='//www.gpai.net/sf\item2.do?Web_Item_ID=53260'>`
# ——**协议相对 + 反斜杠路径**，两种都要收，否则一条都解析不到。
_ITEM_LINK_RE = re.compile(
    r"""href=['"]//www\.gpai\.net/sf[/\\]item2\.do\?Web_Item_ID=(\d+)['"]""",
    re.IGNORECASE,
)
# 详情页成交价：`<span>成交价：</span><b class='price-red'>195899</b>`
_DEAL_PRICE_RE = re.compile(
    r"<span>\s*成交价：?\s*</span>\s*<b[^>]*class=['\"]?price-red['\"]?[^>]*>\s*([0-9][0-9,]*(?:\.[0-9]+)?)\s*</b>"
)
# 详情页起拍价：`<td>起拍价：<span id="Price_Start">167899</span>元</td>`
_START_PRICE_RE = re.compile(
    r'id=["\']Price_Start["\'][^>]*>\s*([0-9][0-9,]*(?:\.[0-9]+)?)'
)
# 详情页评估价：`<td> 评估价：5,666,152 元</td>`（跨行、带千分位、无独立 id）
_APPRAISAL_RE = re.compile(
    r"评估价：?\s*([0-9][0-9,]*(?:\.[0-9]+)?)"
)
_TITLE_RE = re.compile(r"<title>(.*?)</title>", re.IGNORECASE | re.DOTALL)
# 状态徽章：`<label class='status-badge'>正在拍卖</label>` / `<span class="fl time-txt">已结束</span>`
_STATUS_RE = re.compile(
    r"(?:status-badge['\"]?[^>]*>|time-txt['\"]?[^>]*>)\s*([^<]{1,12}?)\s*<",
)
# 结束时间：`<span>结束时间：2026-11-20 11:27:49</span>`
_END_TIME_RE = re.compile(r"结束时间：\s*(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2})")
# 吨位：标题里「3T汽油液力式叉车」「2吨电动托盘车」。
_TONNAGE_RE = re.compile(r"(\d+(?:\.\d+)?)\s*(?:吨|T(?![A-Za-z]))", re.IGNORECASE)
# 报废/无电池线索（与京东同一组词）：清洗规则②的判据字段必须由适配器填，
# 否则「废铁价 ≠ 使用价值」这条规则在结构化采集面上永远不会命中。
_SCRAPPED_MARKERS = ("报废", "废铁", "拆解", "注销", "已报废")
_NO_BATTERY_MARKERS = ("无电池", "不含电池", "缺电池", "电池缺失", "无电瓶")

# 验证码页的硬指纹（实测 seccaptcha.haplat.net 的滑块页）。
_CAPTCHA_MARKERS = (
    "seccaptcha.haplat.net",
    "captchaPage",
    "rightValidate_cont",
    "infoString",
)


class CaptchaBlocked(RuntimeError):
    """被反爬验证码拦截。必须上抛，不能降级成空样本（见模块 docstring）。"""


def _looks_like_captcha(html: str) -> bool:
    """验证码页识别：命中任一硬指纹即判拦截。

    不能只按体积判（虽然实测验证码页 1,774 B、正常页 99 KB+）——体积是偶然属性，
    指纹才是语义。
    """
    text = html or ""
    return sum(1 for marker in _CAPTCHA_MARKERS if marker in text) >= 2


def _price(text: str | None) -> float | None:
    if not text:
        return None
    try:
        value = float(str(text).replace(",", "").strip())
    except ValueError:
        return None
    return value if value > 0 else None


def _first_price(pattern: re.Pattern[str], html: str) -> float | None:
    """取第一个匹配组的数值（无匹配 / 非数字 ⇒ None）。"""
    match = pattern.search(html or "")
    return _price(match.group(1)) if match else None


def _tonnage(text: str) -> float | None:
    match = _TONNAGE_RE.search(text or "")
    if not match:
        return None
    try:
        return float(match.group(1))
    except ValueError:
        return None


def _title_of(html: str) -> str:
    match = _TITLE_RE.search(html or "")
    if not match:
        return ""
    # 标题形如「… - 拍品详情 - 公拍网」，去掉站点后缀。
    return match.group(1).split(" - ")[0].strip()


def _status_of(html: str) -> str:
    match = _STATUS_RE.search(html or "")
    return match.group(1).strip() if match else ""


def parse_sf_listing(html: str, list_url: str) -> list[DiscoveredAnnouncement]:
    """司法频道列表页 → 标的详情页 URL（去重保序）。

    列表页条目里已有起拍价/评估价/状态，但**不在这里解析成样本**：发现层的产出是
    URL，样本由详情页解析——保持「列表只发现、详情才落样本」的一致口径，
    否则同一标的从列表进来的样本和从详情进来的样本字段集不同，去重也难对齐。
    """
    if not html or not html.strip():
        return []
    seen: set[str] = set()
    items: list[DiscoveredAnnouncement] = []
    for match in _ITEM_LINK_RE.finditer(html):
        item_id = match.group(1)
        if item_id in seen:
            continue
        seen.add(item_id)
        items.append(
            DiscoveredAnnouncement(
                url=urljoin(_SF_BASE, f"item2.do?Web_Item_ID={item_id}"),
                title="",
                publish_date="",
            )
        )
    return items


def parse_sf_detail(html: str, source_url: str) -> list[RawSample]:
    """司法频道详情页 → 一条 raw 样本。

    三种形态（实测）：
      - 在拍：只有起拍价 + 评估价 ⇒ `raw_price_kind='listing'`；
      - 已成交：另有成交价 ⇒ `raw_price_kind='deal'`；
      - 已取消：状态「已结束」但 d-m-price 里仍只有起拍价 ⇒ 按 listing 落，
        不能因为「已结束」就当成成交（实测 52500：`拍卖/变卖取消`）。
    """
    if _looks_like_captcha(html):
        raise CaptchaBlocked(
            f"公拍网返回滑动验证码（UA 已被拉黑）：{source_url}；"
            "该响应必须按抓取失败处理，不能解析成空样本"
        )
    if not html or not html.strip():
        return []

    title = _title_of(html)
    deal_price = _first_price(_DEAL_PRICE_RE, html)
    start_price = _first_price(_START_PRICE_RE, html)
    appraisal = _first_price(_APPRAISAL_RE, html)
    status = _status_of(html)
    tonnage = _tonnage(title)

    raw_price = deal_price if deal_price is not None else start_price
    raw_price_kind = "deal" if deal_price is not None else "listing"

    specs: dict[str, Any] = {
        "title": title,
        "status": status,
        "start_price": start_price,
        "deal_price": deal_price,
    }
    if tonnage is not None:
        specs["tonnage"] = tonnage
    if any(marker in title for marker in _SCRAPPED_MARKERS):
        specs["scrapped"] = True
    if any(marker in title for marker in _NO_BATTERY_MARKERS):
        specs["has_battery"] = False

    confidence: dict[str, float] = {}
    if raw_price is not None:
        # 价格来自服务端渲染的固定标记（id="Price_Start" / class="price-red"）⇒ 直取。
        confidence["raw_price"] = 1.0
    if tonnage is not None:
        confidence["tonnage"] = 0.9

    end_match = _END_TIME_RE.search(html)
    auction_date: datetime | None = None
    if end_match:
        try:
            auction_date = datetime.strptime(
                end_match.group(1).replace("T", " "), "%Y-%m-%d %H:%M:%S"
            ).replace(tzinfo=timezone.utc)
        except ValueError:
            auction_date = None

    return [
        RawSample(
            channel=CHANNEL_GPAI,
            source_url=source_url,
            raw_description={
                "title": title,
                "body": title,
                "status": status,
                "start_price": start_price,
                "deal_price": deal_price,
                "appraisal_price": appraisal,
            },
            raw_price=raw_price,
            raw_price_kind=raw_price_kind,
            appraisal_price=appraisal,
            auction_date=auction_date,
            structured_specs=specs,
            field_confidence=confidence,
        )
    ]


def parse_zc_listing(payload: str, list_url: str) -> list[DiscoveredAnnouncement]:
    """资产频道列表接口（JSON）→ 标的 URL。

    平台能力边界：列表接口的 pageSize/pageNum/keyword 全被忽略，只能拿到当前一页。
    所以这里产出的是「这一页里的标的」，不含历史。
    """
    import json

    if not payload or not payload.strip():
        return []
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise ValueError(f"公拍网资产频道列表响应不是合法 JSON: {exc}") from exc
    rows = ((data.get("data") or {}).get("list")) or []
    items: list[DiscoveredAnnouncement] = []
    seen: set[str] = set()
    for row in rows:
        item_id = str(row.get("itemId") or row.get("id") or "")
        if not item_id or item_id in seen:
            continue
        seen.add(item_id)
        items.append(
            DiscoveredAnnouncement(
                url=urljoin(_ZC_BASE, f"zc/detail?id={item_id}"),
                title=str(row.get("itemName") or ""),
                # 资产频道给的是开拍时间；无发布日期 ⇒ 结构化面 keep_undated 放行。
                publish_date="",
            )
        )
    return items


def parse_zc_detail(html: str, source_url: str) -> list[RawSample]:
    """资产频道详情 —— **不可用**，必须显式失败而不是返回空。

    实测 `zc/detail?id=<id>` 是 1,450 B 的 Vue SPA 空壳，所有详情接口都返回
    `info为空` / `data:null` / `未登录`。既然拿不到成交价与评估价，就不能假装
    能落样本：返回空列表会让编排层把它当「解析出 0 条」静默跳过，问题被掩盖。
    """
    raise ValueError(
        f"公拍网资产频道详情不可用（SPA 空壳 + 详情接口全废）：{source_url}；"
        "该频道只能通过列表接口落「在拍标的 + 起拍价」"
    )


def parse_listing(payload: str, list_url: str) -> list[DiscoveredAnnouncement]:
    """列表入口分派：司法频道是 HTML，资产频道是 JSON 接口。

    两个频道共用同一个采集面（同一个 `channel='gpai'`，因为它们是同一个平台的
    两种标的口径），但列表形态完全不同——按 list_url 是否落在 `zc.` 上分派。
    """
    if "zc.gpai.net" in list_url:
        return parse_zc_listing(payload, list_url)
    return parse_sf_listing(payload, list_url)


def fetch_detail(url: str, fetch: Fetch) -> list[RawSample]:
    """详情入口分派：司法频道抓页面解析，资产频道显式失败（见 parse_zc_detail）。"""
    if "zc.gpai.net" in url:
        return parse_zc_detail(fetch(url), url)
    return parse_sf_detail(fetch(url), url)


SPEC = register(
    SourceSpec(
        channel=CHANNEL_GPAI,
        base_url=_SF_BASE,
        # 司法频道在 www，资产频道在 zc，已结束列表在 s——三处各自核验 robots。
        origins=("https://www.gpai.net", "https://zc.gpai.net", "https://s.gpai.net"),
        url_markers=("gpai.net/sf/", "gpai.net/sf\\", "zc.gpai.net/"),
        parse_listing=parse_listing,
        fetch_detail=fetch_detail,
        structured=True,
        keep_undated=True,
    )
)
