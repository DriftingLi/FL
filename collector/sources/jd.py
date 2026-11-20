"""京东拍卖（paimai.jd.com）采集面适配器（#2679 判据 1/3/4）。

## 为什么必须走 JSONP 而不是解析标的页

实测标的页 `https://paimai.jd.com/<paimaiId>` 是 SPA 空壳（78,393 B，HTML 里
`dealPrice` / `finalPrice` / `currentPrice` / `__INITIAL_STATE__` 一个都没有），
价格全部由前端调接口后渲染。所以详情必须打 JSONP：

    https://api.m.jd.com/?appid=paimai&functionId=getProductBasicInfo&body={"paimaiId":"<id>"}&loginType=3
    https://api.m.jd.com/?appid=paimai&functionId=getPaimaiRealTimeData&body={"paimaiId":"<id>"}&loginType=3

实测可用接口**只有这两个**：`getProductBidInfo` / `getBidList` / `getProductDealInfo` /
`getPaimaiResult` / `getProductInfo` / `getAuctionResult` 全部返回
`{"code":"2","echo":"the current API does not exist"}`——不要猜接口名。

## 发现入口用公告页（服务端渲染）

`https://paimai.jd.com/notice/<noticeId>` 是服务端渲染的，内含
`<input type="hidden" class="albumProductInfosForId" value="<paimaiId>">` 逐个列出
该公告下的标的。这是无需站内搜索的合法发现入口（站内搜索页未探明、且会扩大采集面）。

## 判据④（破产通道）的真实形态 —— 与原票假设不同，按实测修正

原票假设「破产通道不公示确认书」可作判据。实测 7 个样本（含已结束的 2026-01 场次）
**全部** `auctionStatus=2`、`confirmationUrl=""`，包括正常司法拍卖场次：

  - `auctionStatus=2` 是常态值，不能用来判「已成交」；
  - `confirmationUrl` 空串也是常态，不能用来判「破产通道」。

真实可用的判据是**评估价是否为零**：破产/资产处置通道（auctionType=5、businessType=1、
platType=4、publishSource=9）的 `assessmentPrice` 实测全为 0.0（7/7），而清算价字段
`liquidationPriceText` / `liquidationDisplayText` 全为空串。

所以判据④的落地是：**评估价为 0 时照常落成交价，只是跳过溢价比较**。这不是妥协——
清洗规则⑦ `_rule_premium` 本来就只在 `appraisal` 为真值时才比较（`if price is not None
and appraisal:`），所以 0 评估价天然不会触发除零或误判。本适配器负责把 0 归一成
`None`（「源站不提供评估价」而不是「评估价为零元」），让下游不必猜。

## 成交价怎么定

`getPaimaiRealTimeData.currentPrice` 是当前最高应价。已结束场次里它就是成交价
（实测 310117853：currentPrice=15000、bidCount=1、bidList 一条 15000）。
`bidCount=0` 的样本没有应价 ⇒ 无成交价 ⇒ 只落起拍价（`raw_price_kind='listing'`）。

## robots

京东三处 host 都**拿不到有效 robots.txt**（实测：paimai.jd.com → 302 到
www.jd.com/error.html；api.m.jd.com → 返回 JSON 错误体；www.jd.com → 302 到首页 HTML）。
按 ADR-0101 决策二必须 fail-closed ⇒ 采集必须显式 `--allow-robots-override`，
且每次请求都记 `robots_override` 审计。
"""

from __future__ import annotations

import json
import re
from datetime import datetime, timezone
from typing import Any
from urllib.parse import quote, urljoin

from ..backfill import DiscoveredAnnouncement
from ..models import CHANNEL_JD, RawSample
from .base import SourceSpec, register

# 两个实测可用的 JSONP 接口（换一个就是 the current API does not exist）。
_FUNCTION_BASIC = "getProductBasicInfo"
_FUNCTION_REALTIME = "getPaimaiRealTimeData"
_API_BASE = "https://api.m.jd.com/"
_NOTICE_BASE = "https://paimai.jd.com/notice/"
_ITEM_BASE = "https://paimai.jd.com/"

# 公告页里的标的 id：`<input type="hidden" class="albumProductInfosForId" value="309492343">`
_ALBUM_ID_RE = re.compile(
    r'<input[^>]*class="[^"]*albumProductInfosForId[^"]*"[^>]*value="(\d+)"',
    re.IGNORECASE,
)
# 属性顺序不保证，补一条反向的（value 在前、class 在后）。
_ALBUM_ID_RE_REV = re.compile(
    r'<input[^>]*value="(\d+)"[^>]*class="[^"]*albumProductInfosForId',
    re.IGNORECASE,
)
_NOTICE_ID_RE = re.compile(r"/notice/(\d+)")
# 标题里的吨位：实测「林德T20AP-2吨电动托盘车」「3T汽油液力式叉车」两种写法。
_TONNAGE_RE = re.compile(r"(\d+(?:\.\d+)?)\s*(?:吨|T(?![A-Za-z]))", re.IGNORECASE)
# 报废/无电池线索：京东破产与资产处置通道的标题几乎全是这种形态（实测 7/7）。
#
# 为什么必须在适配器里判：清洗规则②（`_rule_scrapped`，废铁价 ≠ 使用价值）的判据是
# `structured_specs.scrapped` / `has_battery is False`——**字段不填，规则就永远不会命中**。
# 而京东这批样本恰恰是规则②最该拦的那类：报废资产按废铁计价，把它当成市场成交价
# 去校准残值率，会把整条校准链拉偏。
_SCRAPPED_MARKERS = ("报废", "废铁", "拆解", "注销", "已报废")
_NO_BATTERY_MARKERS = ("无电池", "不含电池", "缺电池", "电池缺失", "无电瓶")


def _api_url(function_id: str, paimai_id: str) -> str:
    """构造 JSONP 请求 URL（body 是 JSON 串，必须整体 URL 编码）。"""
    body = json.dumps({"paimaiId": paimai_id}, separators=(",", ":"))
    return (
        f"{_API_BASE}?appid=paimai&functionId={function_id}"
        f"&body={quote(body, safe='')}&loginType=3"
    )


def _parse_jsonp(text: str) -> dict[str, Any]:
    """解析 JSONP 响应。实测接口返回纯 JSON（不是 `callback(...)` 包装），但两种都收。

    非 JSON / `code != 0` 一律抛错——静默返回空 dict 会让上层把「接口拒绝」
    当成「该标的无数据」，落一批空行。
    """
    payload = (text or "").strip()
    if not payload:
        raise ValueError("京东接口返回空响应")
    # 兼容 callback 包装：取第一个 '(' 到最后一个 ')' 之间的内容。
    if not payload.startswith(("{", "[")):
        start, end = payload.find("("), payload.rfind(")")
        if start >= 0 and end > start:
            payload = payload[start + 1 : end]
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise ValueError(f"京东接口响应不是合法 JSON: {exc}") from exc
    if not isinstance(data, dict):
        raise ValueError("京东接口响应不是 JSON 对象")
    code = data.get("code")
    if code not in (0, "0"):
        raise ValueError(f"京东接口拒绝（code={code!r}）：{data.get('echo') or data.get('message')}")
    return data.get("data") or {}


def parse_notice(html: str, notice_url: str = "") -> list[DiscoveredAnnouncement]:
    """公告页 → 该公告下每个标的的标的页 URL。

    公告页是**发现入口**：一个公告（专场）下有 N 个标的。列表页条目里带标题时用它，
    没有就留空（标题由详情接口补——`getProductBasicInfo.title` 才是权威标题）。
    """
    if not html or not html.strip():
        return []
    ids: list[str] = []
    for pattern in (_ALBUM_ID_RE, _ALBUM_ID_RE_REV):
        for match in pattern.finditer(html):
            pid = match.group(1)
            if pid not in ids:
                ids.append(pid)
    return [
        DiscoveredAnnouncement(
            url=f"{_ITEM_BASE}{pid}",
            title="",
            # 公告页不给每个标的的成交日期；京东标的页也没有发布日期。
            # 结构化采集面靠 keep_undated=True 放行（见 SourceSpec 说明）。
            publish_date="",
        )
        for pid in ids
    ]


def _ms_to_date(value: Any) -> datetime | None:
    """京东的时间戳是毫秒 epoch（实测 1768442400000 → 2026-01-15 02:00 UTC+8）。"""
    if value in (None, "", 0):
        return None
    try:
        millis = int(value)
    except (TypeError, ValueError):
        return None
    if millis <= 0:
        return None
    return datetime.fromtimestamp(millis / 1000, tz=timezone.utc)


def _as_price(value: Any) -> float | None:
    """价格归一：0 / 空串 / "无" 一律 None（源站不提供，不是零元）。"""
    if value is None:
        return None
    if isinstance(value, (int, float)):
        return float(value) if float(value) > 0 else None
    text = str(value).strip().replace(",", "").replace("元", "")
    if not text or text in ("无", "-", "面议"):
        return None
    try:
        number = float(text)
    except ValueError:
        return None
    return number if number > 0 else None


def _tonnage_from_title(title: str) -> float | None:
    match = _TONNAGE_RE.search(title or "")
    if not match:
        return None
    try:
        return float(match.group(1))
    except ValueError:
        return None


def _jd_id_of(url: str) -> str:
    """从标的页 URL 取 paimaiId（`https://paimai.jd.com/310055258`）。"""
    tail = url.rstrip("/").rsplit("/", 1)[-1]
    if not tail.isdigit():
        raise ValueError(f"京东标的页 URL 取不出 paimaiId：{url}")
    return tail


def _build_sample(
    paimai_id: str,
    basic: dict[str, Any],
    realtime: dict[str, Any] | None,
) -> RawSample:
    """把两个接口的 data 合成一条 raw 样本。

    字段置信度按「取值机制」给：JSONP 字段直取 ⇒ 1.0；从标题正则推的吨位 ⇒ 0.9
    （标题是人写的，可能省略或写错）；缺失字段**不写置信度**（见
    `extract.decisions.claimed_low_confidence_reason`：缺席 ≠ 抽取失败）。
    """
    title = str(basic.get("title") or "")
    address = basic.get("productAddressResult") or {}
    province = str(address.get("province") or "")
    city = str(address.get("city") or "")
    county = str(address.get("county") or "")

    start_price = _as_price(basic.get("startPrice"))
    # 评估价：实测破产/资产通道全为 0.0 ⇒ 归一成 None（「不提供」而非「零元」）。
    appraisal_price = _as_price(basic.get("assessmentPrice"))

    # 成交价：currentPrice 是当前最高应价；无应价（bidCount=0）时它不是成交价。
    deal_price: float | None = None
    bid_count = 0
    if realtime:
        bid_count = int(realtime.get("bidCount") or 0)
        if bid_count > 0:
            deal_price = _as_price(realtime.get("currentPrice"))

    tonnage = _tonnage_from_title(title)

    specs: dict[str, Any] = {
        "title": title,
        "province": province,
        "city": city,
        "county": county,
        "auction_type": basic.get("auctionType"),
        "business_type": basic.get("businessType"),
        "platform_type": basic.get("platType"),
        "publish_source": basic.get("publishSource"),
        "bid_count": bid_count,
        "paimai_id": paimai_id,
    }
    if tonnage is not None:
        specs["tonnage"] = tonnage
    # 报废/无电池：只有**明确命中**才写字段，不命中就留空（「没提到」≠「车况良好」）。
    # 清洗规则②据此剔除——它必须能被触发，否则废铁价会被当成市场成交价。
    if any(marker in title for marker in _SCRAPPED_MARKERS):
        specs["scrapped"] = True
    if any(marker in title for marker in _NO_BATTERY_MARKERS):
        specs["has_battery"] = False

    confidence: dict[str, float] = {"raw_price": 1.0}
    if tonnage is not None:
        # 标题正则推的吨位：低于直取字段，但仍高于抽检阈值（0.9 > 0.75）。
        confidence["tonnage"] = 0.9

    raw_price = deal_price if deal_price is not None else start_price
    raw_price_kind = "deal" if deal_price is not None else "listing"

    return RawSample(
        channel=CHANNEL_JD,
        source_url=f"{_ITEM_BASE}{paimai_id}",
        raw_description={
            "title": title,
            "body": title,
            "start_price": start_price,
            "appraisal_price": appraisal_price,
            "deal_price": deal_price,
            "bid_count": bid_count,
            "location": "".join(part for part in (province, city, county) if part),
        },
        raw_price=raw_price,
        raw_price_kind=raw_price_kind,
        appraisal_price=appraisal_price,
        auction_date=_ms_to_date((realtime or {}).get("endTime") or basic.get("endTime")),
        structured_specs=specs,
        field_confidence=confidence,
    )


def fetch_detail(url: str, fetch) -> list[RawSample]:
    """标的页 URL → 一条 raw 样本（打两个 JSONP 接口，不解析标的页 HTML）。

    `realtime` 接口失败只降级为「无成交价」，不让整条样本失败：起拍价与评估价
    来自 basic 接口，已经够落一条 listing 样本。但 basic 失败必须抛——没有价格
    的样本落库只会污染数据。
    """
    paimai_id = _jd_id_of(url)
    basic = _parse_jsonp(fetch(_api_url(_FUNCTION_BASIC, paimai_id)))
    realtime: dict[str, Any] | None = None
    try:
        realtime = _parse_jsonp(fetch(_api_url(_FUNCTION_REALTIME, paimai_id)))
    except (ValueError, OSError):
        # 成交价缺失可接受（该标的可能无应价/接口临时不可用）；起拍价仍在。
        realtime = None
    return [_build_sample(paimai_id, basic, realtime)]


def parse_listing(html: str, list_url: str) -> list[DiscoveredAnnouncement]:
    """列表页（公告页）→ 标的页 URL 列表。"""
    return parse_notice(html, list_url)


def notice_url(notice_id: str | int) -> str:
    """公告页 URL（发现入口；测试与运维都靠它构造 list_url）。"""
    return f"{_NOTICE_BASE}{notice_id}"


SPEC = register(
    SourceSpec(
        channel=CHANNEL_JD,
        base_url="https://paimai.jd.com",
        # 三个 host 各自读 robots、各自 fail-closed：标的页在 paimai.jd.com，
        # JSONP 在 api.m.jd.com，公告页也是 paimai.jd.com。
        origins=("https://paimai.jd.com", "https://api.m.jd.com"),
        url_markers=("paimai.jd.com/notice/", "paimai.jd.com/"),
        parse_listing=parse_listing,
        fetch_detail=fetch_detail,
        # 字段来自 JSONP 结构化响应 ⇒ 跳过 LLM 抽取。
        structured=True,
        # 京东标的没有发布日期（接口只给 startTime/endTime 的 ms 时间戳，且
        # 公告页不给），按年份窗口过滤会把全部标的丢掉 ⇒ 必须放行无日期条目。
        keep_undated=True,
    )
)
