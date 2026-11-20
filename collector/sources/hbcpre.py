"""湖北省级产权交易平台镜像（hbggzyfwpt.cn）采集面适配器（#2680）。

## 为什么这个源值得单独接

它是**唯一同时给出「挂牌价格」与「成交金额」的公开源**。司法拍卖（gpai）只有成交价，
京东破产通道连评估价都是 0——「挂牌→成交」的折扣关系在这些源上无从观测。而这个源的
成交公告页上两个价并排出现（实测样本：评估价 199202 / 挂牌价 199300 / 成交金额 268200），
正好是下游校准检验「清算折扣 δ 基线 0.85」的第一批真配对样本。

## 两个入口，两种形态（实测）

- 挂牌列表 `https://www.hbggzyfwpt.cn/jyxx/whcq/jygg` → 条目链接 `jyggDetail?guid=<uuid>`
- 成交列表 `https://www.hbggzyfwpt.cn/jyxx/whcq/jyjg` → 条目链接 `jyjgDetail?guid=<uuid>`

**两个列表的条目行缺 `</tr>`**（`<td>` 结束直接接下一个 `<tr>`，实测源码如此）。
所以解析器不能靠 `</tr>` 分块——用 `HTMLParser` 按 `<a>` 标签取链接与 title 属性，
不依赖表格结构完整性。

## 两种详情页，表格形态不同（实测）

**挂牌详情 `jyggDetail`**：`<div class="detailNeirong">` 内 4-td 双栏表格，
标签形如 `<td width="20%" align="right">挂牌价格（万元）:</td>`（**带半角冒号**）。
字段：项目编号 / 标的名称 / 项目所属地区 / 挂牌公告期 / **挂牌价格（万元）** /
保证金金额 / 联系人 / 联系方式 / 交易方式 / 相关事项说明 / **源网站链接**。

**成交详情 `jyjgDetail`**：`<div class="table table_cj">` 内 2-td 单栏表格，
标签**不带冒号**：`<td width="210">挂牌价格（元）</td><td>199300</td>`。
字段：项目编号 / 项目名称 / 转让方名称 / **评估价格（元）** / **挂牌价格（元）** /
公告期限 / 公告媒体 / 意向受让方数量 / 交易方式 / 受让方名称 / **成交金额（元）** /
源网站链接。

⇒ **成交详情页一页就有两个价**（判据②「同一样本两价齐备」直接满足），不需要跨页配对。
挂牌详情与成交详情是不同 guid，跨页关联靠「项目编号」（如 202510082）——但既然成交页
自带两价，配对不依赖跨页。

## 单位陷阱：挂牌价是「万元」，成交页两个价是「元」

实测挂牌详情 `挂牌价格（万元）: 2.43` = 24,300 元；成交详情 `挂牌价格（元）: 199300`。
标签里带了单位就必须按标签取单位，不能靠猜——差 10000 倍的价格会让残值率标签全错。
解析时按标签里的「（万元）」/「（元）」判单位并统一折成元。

## 租赁权陷阱：这个源大量发布「承租权 / 租赁权」而非设备

实测当期 9 条成交公告**全是**「宜昌市夷兴大道…三年承租权成交公告」。它们的
「成交金额」是**三年租金总额**、「评估价格」是**年租金评估价**——量纲根本不同
（实测 ratio 3.86~7.33，不是溢价）。若当设备样本落库，会把租金比当成残值率。

适配器在 `structured_specs` 上标记 `lease_right=True`，由清洗规则⑧剔除
（`REASON_LEASE_RIGHT`，见 `collector/clean/rules.py`）。**不在这里直接丢弃**——
丢弃就没有剔除审计，得率报告也看不见「这个源当期 100% 是租赁权」这个事实。

## robots

`https://www.hbggzyfwpt.cn/robots.txt` → **HTTP 404**（Tomcat 的 404 页）。按 ADR-0101
决策二「读不到 robots 即 fail-closed」必须视为全站禁止 ⇒ 采集需显式
`--allow-robots-override` 并记 `robots_override` 审计。这是运维决策，不是实现选择。
"""

from __future__ import annotations

import re
from datetime import datetime, timezone
from html.parser import HTMLParser
from typing import Any
from urllib.parse import urljoin

from ..backfill import DiscoveredAnnouncement
from ..models import CHANNEL_HBCPRE, RawSample
from .base import SourceSpec, html_detail, register

_BASE = "https://www.hbggzyfwpt.cn"
_LIST_JYGG = f"{_BASE}/jyxx/whcq/jygg"
_LIST_JYJG = f"{_BASE}/jyxx/whcq/jyjg"

# 列表条目链接（实测 `/jyxx/whcq/jyggDetail?guid=<uuid>`，绝对/相对都可能）。
_DETAIL_LINK_MARKERS = ("jyggDetail", "jyjgDetail")
# 吨位：标题「一台3T汽油液力式叉车资产处置」。
_TONNAGE_RE = re.compile(r"(\d+(?:\.\d+)?)\s*(?:吨|T(?![A-Za-z]))", re.IGNORECASE)
# 租赁权/承租权：与设备样本量纲不同，必须单独标记后剔除。
_LEASE_MARKERS = ("承租权", "租赁权", "出租", "租金", "房屋出租", "招租")
# 报废/无电池（与京东/gpai 同一组词，供清洗规则②使用）。
_SCRAPPED_MARKERS = ("报废", "废铁", "拆解", "注销", "已报废")
_NO_BATTERY_MARKERS = ("无电池", "不含电池", "缺电池", "电池缺失", "无电瓶")
# 设备相关性线索：这个源是通用产权交易平台，当期条目多为房产/租赁权/非叉车物资。
# 标记出来供下游筛选——**不在这里剔除**（判「不是叉车」需要穷举非叉车品类，
# 判据会随平台内容漂移）；非叉车样本没有品牌/吨位，匹配不会命中，自然不会产出标签。
_EQUIPMENT_MARKERS = ("叉车", "托盘车", "堆高车", "搬运车", "前移式", "牵引车", "电动叉车")
# 发布时间：`<span>发布时间：  2026-04-23 </span>`
_PUBLISH_RE = re.compile(r"发布时间：\s*(\d{4}-\d{2}-\d{2})")
# 挂牌公告期：`2026-04-23 至 2026-05-09`（取起始日作为发布日期兜底）。
_PERIOD_RE = re.compile(r"(\d{4}-\d{2}-\d{2})\s*至\s*(\d{4}-\d{2}-\d{2})")
_TITLE_RE = re.compile(r'<div class="detailTitle">\s*(.*?)\s*</div>', re.DOTALL)


class _LinkParser(HTMLParser):
    """从列表页取 `<a href=... title=...>` 里的详情链接与标题。

    不按 `<tr>` 分块——实测源站列表行缺 `</tr>`（`<td>` 结束直接接下一个 `<tr>`），
    按行分块会漏掉一半条目。
    """

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.items: list[tuple[str, str]] = []

    def handle_starttag(self, tag: str, attrs) -> None:
        if tag != "a":
            return
        attrs_map = dict(attrs)
        href = attrs_map.get("href") or ""
        if not any(marker in href for marker in _DETAIL_LINK_MARKERS):
            return
        self.items.append((href, attrs_map.get("title") or ""))


def _absolute(href: str) -> str:
    """相对链接补成绝对地址（幂等判重的键必须是绝对地址）。"""
    return urljoin(_BASE + "/", href)


def parse_listing(html: str, list_url: str) -> list[DiscoveredAnnouncement]:
    """列表页 → 详情页 URL（去重保序）。挂牌与成交两个列表共用。"""
    if not html or not html.strip():
        return []
    parser = _LinkParser()
    parser.feed(html)
    seen: set[str] = set()
    items: list[DiscoveredAnnouncement] = []
    for href, title in parser.items:
        url = _absolute(href)
        if url in seen:
            continue
        seen.add(url)
        # 发布日期：列表页条目里有 `2026-10-30`，但它在 `<td>` 里、与链接不同层级，
        # 用 HTMLParser 按链接配对会串行。所以留空——结构化采集面 keep_undated 放行，
        # 真正的拍卖日期从详情页的「公告期限 / 发布时间」取。
        items.append(DiscoveredAnnouncement(url=url, title=title, publish_date=""))
    return items


class _TableParser(HTMLParser):
    """把详情页表格解析成「标签 → 值」字典。

    两种详情页的表格形态不同（挂牌是 4-td 双栏、成交是 2-td 单栏），但都是
    「标签单元格 + 值单元格」的成对结构。统一做法：按 `<td>` 收集文本，遇到
    「标签形态」（以冒号结尾，或命中已知标签名）就把它当标签，下一个单元格是值。

    已知标签名清单是**必需的**：成交页标签不带冒号（`挂牌价格（元）`），
    只靠「以冒号结尾」判不出来。
    """

    # 已知标签前缀（去掉单位与冒号后比对）。用「前缀匹配」容忍标签里的单位差异
    # （`挂牌价格（万元）` / `挂牌价格（元）` / `评估价格（元）` / `评估价格`）。
    _KNOWN_LABELS = (
        "项目编号",
        "标的名称",
        "项目名称",
        "项目所属地区",
        "挂牌公告期",
        "挂牌价格",
        "评估价格",
        "保证金金额",
        "联系人",
        "联系方式",
        "交易方式",
        "相关事项说明",
        "源网站链接",
        "转让方名称",
        "公告期限",
        "公告媒体",
        "意向受让方数量",
        "受让方名称",
        "成交金额",
    )

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.fields: dict[str, str] = {}
        self._cell_parts: list[str] = []
        self._in_cell = False
        self._pending_label: str | None = None

    def handle_starttag(self, tag: str, attrs) -> None:
        if tag == "td":
            self._in_cell = True
            self._cell_parts = []

    def handle_endtag(self, tag: str) -> None:
        if tag != "td" or not self._in_cell:
            return
        self._in_cell = False
        text = " ".join("".join(self._cell_parts).split())
        self._absorb(text)

    def handle_data(self, data: str) -> None:
        if self._in_cell:
            self._cell_parts.append(data)

    def _absorb(self, text: str) -> None:
        if not text:
            return
        label = self._label_of(text)
        if label is not None:
            self._pending_label = label
            # 标签单元格偶尔自带值（`项目编号: 202610092`），把冒号后的部分当值。
            _, _, inline = text.partition("：")
            if inline.strip():
                self.fields.setdefault(label, inline.strip())
            return
        if self._pending_label is not None:
            # 值单元格：不覆盖已从标签单元格拿到的内联值（先到先得）。
            self.fields.setdefault(self._pending_label, text)
            self._pending_label = None

    def _label_of(self, text: str) -> str | None:
        normalized = text.rstrip("：:").strip()
        for known in self._KNOWN_LABELS:
            if normalized.startswith(known):
                return known
        return None


def _parse_table(html: str) -> dict[str, str]:
    parser = _TableParser()
    parser.feed(html or "")
    return parser.fields


def _to_yuan(value: str | None, *, unit_hint: str = "") -> float | None:
    """把表格里的价格文本折成「元」。

    单位判定顺序：显式 `unit_hint`（来自标签里的「（万元）」）> 文本自带的「万元」>
    默认按元。**不能靠数值大小猜**——挂牌价 2.43 万元与 2.43 元都可能出现。
    """
    if value is None:
        return None
    text = str(value).strip().replace(",", "").replace("，", "")
    if not text or text in ("-", "无", "面议"):
        return None
    # 单位提示优先：标签写「（万元）」时值就是万元。
    is_wan = "万元" in unit_hint or "万元" in text
    text = text.replace("万元", "").replace("元", "").strip()
    try:
        number = float(text)
    except ValueError:
        return None
    if number <= 0:
        return None
    return number * 10000.0 if is_wan else number


def _title_of(html: str) -> str:
    match = _TITLE_RE.search(html or "")
    if match:
        # 标题里可能含标签（实测有 `<b>`），去标签。
        return re.sub(r"<[^>]+>", "", match.group(1)).strip()
    return ""


def _publish_date(html: str) -> datetime | None:
    match = _PUBLISH_RE.search(html or "") or _PERIOD_RE.search(html or "")
    if not match:
        return None
    try:
        return datetime.strptime(match.group(1), "%Y-%m-%d").replace(tzinfo=timezone.utc)
    except ValueError:
        return None


def _specs_common(title: str, fields: dict[str, str]) -> dict[str, Any]:
    """两种详情页共用的结构化规格：标题线索 + 项目编号。"""
    specs: dict[str, Any] = {"title": title}
    project_no = fields.get("项目编号")
    if project_no:
        # 项目编号是跨页配对的键（挂牌详情与成交详情是不同 guid）。
        specs["project_no"] = project_no
    tonnage_match = _TONNAGE_RE.search(title)
    if tonnage_match:
        try:
            specs["tonnage"] = float(tonnage_match.group(1))
        except ValueError:
            pass
    if any(marker in title for marker in _LEASE_MARKERS):
        specs["lease_right"] = True
    if any(marker in title for marker in _SCRAPPED_MARKERS):
        specs["scrapped"] = True
    if any(marker in title for marker in _NO_BATTERY_MARKERS):
        specs["has_battery"] = False
    if any(marker in title for marker in _EQUIPMENT_MARKERS):
        specs["equipment_relevant"] = True
    return specs


def parse_jygg_detail(html: str, source_url: str) -> list[RawSample]:
    """挂牌详情 → 一条 listing 样本（raw_price = 挂牌价，listing_price 同值）。"""
    if not html or not html.strip():
        return []
    fields = _parse_table(html)
    title = _title_of(html) or fields.get("标的名称", "")
    specs = _specs_common(title, fields)
    # 标签是「挂牌价格（万元）」⇒ 值以万元计。
    listing = _to_yuan(fields.get("挂牌价格"), unit_hint="万元")
    confidence: dict[str, float] = {}
    if listing is not None:
        confidence["raw_price"] = 1.0
    if "tonnage" in specs:
        confidence["tonnage"] = 0.9

    return [
        RawSample(
            channel=CHANNEL_HBCPRE,
            source_url=source_url,
            raw_description={
                "title": title,
                "body": title,
                "fields": fields,
            },
            # 挂牌样本：两个价格列都写挂牌价（列语义恒定，下游不必分情况读）。
            raw_price=listing,
            raw_price_kind="listing",
            listing_price=listing,
            appraisal_price=None,
            auction_date=_publish_date(html),
            structured_specs=specs,
            field_confidence=confidence,
        )
    ]


def parse_jyjg_detail(html: str, source_url: str) -> list[RawSample]:
    """成交详情 → 一条 deal 样本（**两价齐备**：成交金额 + 挂牌价格）。

    判据②就落在这里：成交页同时有「挂牌价格（元）」与「成交金额（元）」，
    两个价落进两列，下游可直接算「成交/挂牌」折扣比。
    """
    if not html or not html.strip():
        return []
    fields = _parse_table(html)
    title = _title_of(html) or fields.get("项目名称", "")
    specs = _specs_common(title, fields)

    # 成交页标签已写明「（元）」⇒ 值就是元，不再乘万。
    deal = _to_yuan(fields.get("成交金额"), unit_hint="元")
    listing = _to_yuan(fields.get("挂牌价格"), unit_hint="元")
    appraisal = _to_yuan(fields.get("评估价格"), unit_hint="元")

    confidence: dict[str, float] = {}
    if deal is not None:
        confidence["raw_price"] = 1.0
    if "tonnage" in specs:
        confidence["tonnage"] = 0.9

    return [
        RawSample(
            channel=CHANNEL_HBCPRE,
            source_url=source_url,
            raw_description={
                "title": title,
                "body": title,
                "fields": fields,
            },
            raw_price=deal,
            raw_price_kind="deal",
            # 判据②：挂牌价与成交金额在同一条样本上齐备。
            listing_price=listing,
            appraisal_price=appraisal,
            auction_date=_publish_date(html),
            structured_specs=specs,
            field_confidence=confidence,
        )
    ]


def fetch_detail(url: str, fetch) -> list[RawSample]:
    """详情入口分派：挂牌详情 vs 成交详情（URL 里带 jyggDetail / jyjgDetail）。"""
    html = fetch(url)
    if "jyjgDetail" in url:
        return parse_jyjg_detail(html, url)
    if "jyggDetail" in url:
        return parse_jygg_detail(html, url)
    raise ValueError(
        f"无法识别的湖北产权平台详情 URL（既不是 jyggDetail 也不是 jyjgDetail）：{url}"
    )


SPEC = register(
    SourceSpec(
        channel=CHANNEL_HBCPRE,
        base_url=_BASE,
        # 实测 robots.txt 是 404 ⇒ fail-closed；单 host。
        origins=(_BASE,),
        url_markers=("hbggzyfwpt.cn/jyxx/whcq/jygg", "hbggzyfwpt.cn/jyxx/whcq/jyjg"),
        parse_listing=parse_listing,
        fetch_detail=fetch_detail,
        # 字段来自服务端渲染的固定表格 ⇒ 跳过 LLM 抽取。
        structured=True,
        # 列表页不给每个条目的发布日期（日期在另一个 td 里，与链接不同层级）；
        # 真正日期从详情页的「发布时间 / 公告期限」取。无日期不得被年份窗口丢掉。
        keep_undated=True,
        # 挂牌与成交两个入口都是默认发现入口（一条命令两种列表都抓）。
        default_list_urls=(_LIST_JYGG, _LIST_JYJG),
    )
)
