"""历史回补批跑编排（#2677）。

第一性原理：回补 = 「发现 URL 集合」×「对每个 URL 跑一遍既有 batch 管线」。
本票不重写管线（#2672–#2676 已建），只加两层：

  - **发现层**：列表页 HTML → `DiscoveredAnnouncement`，纯函数、不碰网络也不碰 PG；
  - **编排层**：按 URL 逐个走 `run_batch` 的同一套阶段，用 `source_url` 判重实现幂等。

所以「一条命令完成历史回补」= CLI 的 `backfill` 子命令：发现 → 过滤年份 →
跳过已入库 → 逐个抓详情页 → 落库 → 出得率报告。

得率报告记录为**事实**，不设硬门槛（#2662 已裁）：只统计各清洗状态与各剔除原因
的计数和占比，不判绿红。
"""

from __future__ import annotations

from dataclasses import dataclass
from html.parser import HTMLParser
from typing import Any, Iterable
from urllib.parse import urljoin

BASE = "https://www.rmfysszc.gov.cn"

# 回补年份窗口（spec #2671：2015–2026）。
BACKFILL_START_YEAR = 2015
BACKFILL_END_YEAR = 2026


def daemon_end_year(now_year: int | None = None) -> int:
    """增量 daemon 的窗口上界 = 当前年（不是写死的 2026）。

    回补是一次性作业，窗口固定合理；daemon 是常驻服务，若上界写死 2026，
    2027-01-01 起每一轮都会「成功但发现 0 条」——日志全绿、库里不再增长，
    没有任何报错。窗口必须跟着当下走。
    """
    if now_year is None:
        from datetime import datetime, timezone

        now_year = datetime.now(timezone.utc).year
    return max(BACKFILL_END_YEAR, int(now_year))


_DETAIL_MARKERS = ("rm_xmdetail", "rm_gzgsdetail")


@dataclass(frozen=True)
class DiscoveredAnnouncement:
    """发现层产出：公告 URL + 标题 + 发布日期（YYYY-MM-DD，可能为空）。"""

    url: str
    title: str
    publish_date: str


class _ListParser(HTMLParser):
    """从公告列表页表格里抓 (url, title, publish_date) 三元组。

    结构（实测 www1.rmfysszc.gov.cn/Agency/pmgg.aspx）：每行三格
    ——标题链（a href 指向 rm_xmdetail）、法院名、发布日期（YYYY.MM.DD）。
    解析只认「a href 含详情路径」这一条硬信号，其余按行内位置就近取。
    """

    def __init__(self) -> None:
        super().__init__()
        self.items: list[DiscoveredAnnouncement] = []
        self._in_row = False
        self._cells: list[str] = []
        self._cell_parts: list[str] = []
        self._href = ""
        self._title_attr = ""

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        a = {k: (v or "") for k, v in attrs}
        if tag == "tr":
            self._in_row = True
            self._cells = []
            self._href = ""
            self._title_attr = ""
        elif tag == "td" and self._in_row:
            self._cell_parts = []
        elif tag == "a" and self._in_row:
            href = a.get("href", "")
            if any(marker in href for marker in _DETAIL_MARKERS):
                self._href = href
                self._title_attr = a.get("title", "")

    def handle_data(self, data: str) -> None:
        text = data.strip()
        if self._in_row and text:
            self._cell_parts.append(text)

    def handle_endtag(self, tag: str) -> None:
        if tag == "td" and self._in_row:
            self._cells.append(" ".join(self._cell_parts).strip())
        elif tag == "tr" and self._in_row:
            self._finish_row()
            self._in_row = False

    def _finish_row(self) -> None:
        if not self._href:
            return
        title = self._title_attr or (self._cells[0] if self._cells else "")
        publish = ""
        for cell in self._cells:
            if len(cell) == 10 and cell[4] == "." and cell[7] == ".":
                publish = cell.replace(".", "-")
                break
        self.items.append(
            DiscoveredAnnouncement(
                url=urljoin(BASE, self._href),
                title=title,
                publish_date=publish,
            )
        )


def parse_announcement_list(html: str) -> list[DiscoveredAnnouncement]:
    """解析公告列表页，返回去重后的公告条目（保持页面顺序）。"""
    if not html or not html.strip():
        return []
    parser = _ListParser()
    parser.feed(html)
    seen: set[str] = set()
    unique: list[DiscoveredAnnouncement] = []
    for item in parser.items:
        if item.url in seen:
            continue
        seen.add(item.url)
        unique.append(item)
    return unique


def _year_of(item: DiscoveredAnnouncement) -> int | None:
    if len(item.publish_date) >= 4 and item.publish_date[:4].isdigit():
        return int(item.publish_date[:4])
    return None


def filter_by_year(
    items: Iterable[DiscoveredAnnouncement],
    *,
    start_year: int = BACKFILL_START_YEAR,
    end_year: int = BACKFILL_END_YEAR,
) -> list[DiscoveredAnnouncement]:
    """只留发布日期在窗口内的公告；无日期的丢弃（无法归年，不进回补）。"""
    kept: list[DiscoveredAnnouncement] = []
    for item in items:
        year = _year_of(item)
        if year is None:
            continue
        if start_year <= year <= end_year:
            kept.append(item)
    return kept


def select_new_announcements(
    discovered: Iterable[DiscoveredAnnouncement],
    *,
    existing_urls: Iterable[str],
) -> list[DiscoveredAnnouncement]:
    """幂等判重：已入库 URL 跳过，批内重复 URL 折叠（保留首次出现）。"""
    existing = set(existing_urls)
    seen: set[str] = set()
    new: list[DiscoveredAnnouncement] = []
    for item in discovered:
        if item.url in existing or item.url in seen:
            continue
        seen.add(item.url)
        new.append(item)
    return new


@dataclass(frozen=True)
class YieldReport:
    """回补得率报告：各清洗状态计数 + 各剔除原因计数。记录为事实，不设门槛。"""

    total: int
    by_status: dict[str, int]
    by_reason: dict[str, int]
    # 本轮抓取失败的公告 URL（毒丸隔离的产物）：一条坏的不能挡住后面的。
    failed_urls: tuple[str, ...] = ()

    @property
    def failed_items(self) -> int:
        return len(self.failed_urls)

    @property
    def yield_rate(self) -> float | None:
        """清洗得率 = cleaned / total。无样本时 None（不是 0，避免误读成「全废」）。"""
        if self.total == 0:
            return None
        return self.by_status.get("cleaned", 0) / self.total

    def render(self) -> str:
        if self.total == 0:
            head = "回补得率报告：本轮无样本"
            if self.failed_urls:
                head += f"（{self.failed_items} 条公告抓取失败）"
            return head
        lines = [
            f"回补得率报告：{self.total} 条样本",
            f"  清洗得率：{self.yield_rate:.1%}（cleaned {self.by_status.get('cleaned', 0)} / {self.total}）",
            "  按清洗状态：" + "，".join(
                f"{status} {count}" for status, count in sorted(self.by_status.items())
            ),
        ]
        if self.by_reason:
            lines.append("  按剔除原因：")
            for reason, count in sorted(self.by_reason.items(), key=lambda kv: -kv[1]):
                share = count / self.total
                lines.append(f"    {reason}: {count}（{share:.1%}）")
        else:
            lines.append("  按剔除原因：无剔除")
        if self.failed_urls:
            lines.append(f"  抓取失败：{self.failed_items} 条（不阻塞其它公告，下次再试）")
            for url in self.failed_urls[:5]:
                lines.append(f"    {url}")
            if self.failed_items > 5:
                lines.append(f"    …另有 {self.failed_items - 5} 条")
        lines.append("  （得率记录为事实，不设硬门槛）")
        return "\n".join(lines)


def build_yield_report(
    rows: Iterable[dict[str, Any]],
    *,
    failed_urls: Iterable[str] = (),
) -> YieldReport:
    """从落库行（含 clean_status / rejection_reason）统计得率，附本轮抓取失败清单。"""
    by_status: dict[str, int] = {}
    by_reason: dict[str, int] = {}
    total = 0
    for row in rows:
        total += 1
        status = row.get("clean_status") or "unknown"
        by_status[status] = by_status.get(status, 0) + 1
        reason = row.get("rejection_reason")
        if reason:
            by_reason[reason] = by_reason.get(reason, 0) + 1
    return YieldReport(
        total=total,
        by_status=by_status,
        by_reason=by_reason,
        failed_urls=tuple(failed_urls),
    )
