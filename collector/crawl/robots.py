"""robots.txt 解析与遵从（#2677 判据 2 的合规前提）。

第一性原理：站点用 robots.txt 明示「不要采」时，**诚实 UA 的自动化访问就不再合规**
——诚实 UA 的代价正是「你被认出来时，对方已经明确说过不行」。所以默认必须遵从
robots；不遵从只能是人工显式放行（`allow_override=True`），且这个「绕过」事实必须
能被调用方拿到（`RobotsDecision.overridden`）并落进请求日志审计，不能是静默 True。

实现范围只覆盖 RFC 9309 里我们真正需要的那几条：
  - `User-agent` 分组，具体 UA 段优先于 `*` 段；
  - `Disallow` / `Allow` 前缀匹配，**最长前缀胜**（同长度时 Allow 胜）；
  - 空 `Disallow:` 表示放行；
  - 无匹配分组 ⇒ 无规则 ⇒ 允许。

不做通配符（`*` / `$`）匹配：本站在 robots 里没有用到，加进来只增加没被测过的分支。
"""

from __future__ import annotations

from dataclasses import dataclass
from urllib.parse import urlparse

# 站点的 robots.txt 对我们的段：这里不硬编码站点规则，只在需要时读真文件。


@dataclass(frozen=True)
class RobotsDecision:
    """一次访问判定的结果。overridden=True 表示「robots 禁止，但被人工放行」。"""

    allowed: bool
    robots_disallowed: bool
    overridden: bool


class RobotsDisallowed(RuntimeError):
    """robots.txt 禁止访问该路径且未显式放行。"""


def robots_denied_message(url: str) -> str:
    """robots 拒绝/跨 host 未核验时的统一提示（单一真相，避免两处手抄漂移）。"""
    return (
        f"robots.txt 禁止访问 {url} 或该 host 未核验：默认遵从；如确需采集，"
        "请显式传入 --allow-robots-override（会记审计日志）"
    )


@dataclass(frozen=True)
class _Rule:
    allow: bool
    pattern: str


def _ua_matches(section_ua: str, user_agent: str) -> bool:
    """robots 的 UA 匹配是子串、大小写不敏感（RFC 9309 §2.2.1）。"""
    return section_ua.strip().lower() in user_agent.lower()


def _parse_sections(text: str) -> list[tuple[list[str], list[_Rule]]]:
    """把 robots.txt 解析成 [(agents, rules)]，保持文件顺序。"""
    sections: list[tuple[list[str], list[_Rule]]] = []
    agents: list[str] = []
    rules: list[_Rule] = []

    def flush() -> None:
        nonlocal agents, rules
        if agents:
            sections.append((agents, rules))
        agents, rules = [], []

    for raw in text.splitlines():
        line = raw.split("#", 1)[0].strip()
        if not line or ":" not in line:
            continue
        field, _, value = line.partition(":")
        field = field.strip().lower()
        value = value.strip()
        if field == "user-agent":
            # 连续多个 User-agent 行属于同一分组；遇到新 agent 前已有规则则先收尾。
            if rules:
                flush()
            agents.append(value)
        elif field in ("allow", "disallow"):
            if not agents:
                continue
            rules.append(_Rule(allow=(field == "allow"), pattern=value))
    flush()
    return sections


def _select_rules(sections: list[tuple[list[str], list[_Rule]]], user_agent: str) -> list[_Rule]:
    """具体 UA 段优先；没有任何匹配段时退回 `*` 段。"""
    specific: list[_Rule] = []
    wildcard: list[_Rule] = []
    for agents, rules in sections:
        if any(a != "*" and _ua_matches(a, user_agent) for a in agents):
            specific.extend(rules)
        elif "*" in agents:
            wildcard.extend(rules)
    return specific if specific else wildcard


def _path_of(url: str) -> str:
    parsed = urlparse(url)
    path = parsed.path or "/"
    return path + (f"?{parsed.query}" if parsed.query else "")


@dataclass(frozen=True)
class RobotsPolicy:
    rules: tuple[_Rule, ...]
    # robots.txt 是按 host 生效的（RFC 9309 §2.2.1）。策略绑定读取它的 host，
    # decide() 遇到别的 host 一律 fail-closed——不能拿 www 的 robots 去放行 www1。
    host: str = ""

    def decide(self, url: str, *, allow_override: bool = False) -> RobotsDecision:
        """判定一次访问；overridden 只有「本禁止但被放行」时才为真。"""
        # host 不匹配 = 未核验该 host 的 robots ⇒ 按禁止处理（fail-closed）。
        host_known = not self.host or urlparse(url).netloc.lower() == self.host.lower()
        disallowed = (not host_known) or self._disallowed(_path_of(url))
        if not disallowed:
            return RobotsDecision(allowed=True, robots_disallowed=False, overridden=False)
        if allow_override:
            return RobotsDecision(allowed=True, robots_disallowed=True, overridden=True)
        return RobotsDecision(allowed=False, robots_disallowed=True, overridden=False)

    def allowed(self, url: str) -> bool:
        return self.decide(url).allowed

    def ensure_allowed(self, url: str, *, allow_override: bool = False) -> RobotsDecision:
        """判并抛——client.fetch 复用它，错误文案只留 robots_denied_message 一处。"""
        decision = self.decide(url, allow_override=allow_override)
        if not decision.allowed:
            raise RobotsDisallowed(robots_denied_message(url))
        return decision

    def _disallowed(self, path: str) -> bool:
        """最长前缀胜；同长度时 Allow 胜（RFC 9309 §2.2.2）。"""
        best_len = -1
        best_allow = False
        for rule in self.rules:
            if rule.pattern == "":
                continue
            if not path.startswith(rule.pattern):
                continue
            length = len(rule.pattern)
            if length > best_len or (length == best_len and rule.allow and not best_allow):
                best_len = length
                best_allow = rule.allow
        if best_len < 0:
            return False
        return not best_allow


def parse_robots(
    text: str, *, user_agent: str, host: str = ""
) -> RobotsPolicy:
    """解析 robots.txt 文本，产出对本 UA 生效、绑定 host 的策略。"""
    sections = _parse_sections(text or "")
    return RobotsPolicy(rules=tuple(_select_rules(sections, user_agent)), host=host)
