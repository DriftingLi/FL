"""rmfysszc 公告解析器（tracer bullet 唯一渠道）。"""

from __future__ import annotations

from html.parser import HTMLParser
from typing import Any
from urllib.parse import urljoin

from ..models import CHANNEL_RMFYSSZC, RawSample

_BASE = "https://www.rmfysszc.com"


class _AnnouncementParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.title = ""
        self.publish_date = ""
        self.body_parts: list[str] = []
        self._tag = ""
        self._capture: str | None = None
        self.canonical = ""

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        a = {k: (v or "") for k, v in attrs}
        self._tag = tag
        classes = a.get("class", "").split()
        if tag == "link" and a.get("rel") == "canonical":
            self.canonical = a.get("href", "")
        elif tag == "meta" and a.get("name") == "publish-date":
            self.publish_date = a.get("content", "")
        elif tag in ("h1", "h2") and "notice-title" in classes:
            self._capture = "title"
        elif tag == "div" and "notice-body" in classes:
            self._capture = "body"

    def handle_data(self, data: str) -> None:
        text = data.strip()
        if not text:
            return
        if self._capture == "title":
            self.title += text
        elif self._capture == "body" and self._tag == "p":
            self.body_parts.append(text)

    def handle_endtag(self, tag: str) -> None:
        if self._capture == "title" and tag in ("h1", "h2"):
            self._capture = None
        elif self._capture == "body" and tag == "div":
            self._capture = None

    def to_sample(self, source_url: str = "") -> RawSample | None:
        """把解析结果封装成一条 raw 样本；无标题无正文时返回 None。"""
        if not self.title and not self.body_parts:
            return None
        raw_description: dict[str, Any] = {
            "title": self.title,
            "publish_date": self.publish_date,
            "body": "\n".join(self.body_parts),
        }
        url = source_url or (
            urljoin(_BASE, self.canonical) if self.canonical else ""
        )
        return RawSample(
            channel=CHANNEL_RMFYSSZC,
            source_url=url,
            raw_description=raw_description,
        )


def parse_announcement(html: str, source_url: str = "") -> list[RawSample]:
    """从一份公告 HTML 解析出 0..n 条 raw 样本。

    `source_url` 显式传入时优先；否则取 HTML 里的 canonical 链接（fixture 的场景）。
    """
    if not html or not html.strip():
        return []
    parser = _AnnouncementParser()
    parser.feed(html)
    sample = parser.to_sample(source_url)
    return [sample] if sample is not None else []
