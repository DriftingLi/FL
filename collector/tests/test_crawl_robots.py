"""robots.txt 遵从的单测（#2677）。

第一性原理：站点用 robots.txt 明示「不要采」时，**诚实 UA 的自动化访问就不再合规**——
诚实 UA 的代价是「你被认出来时，对方已经明确说过不行」。所以默认必须遵从 robots，
不遵从只能是人工显式放行（并留审计），不能是默认路径或静默 fallback。

TDD 红：先于实现写下，钉住：
  - `User-agent: *` + `Disallow: /` ⇒ 全站禁采；
  - 具体 UA 段覆盖通配段（站点给某个爬虫开了口子，我们照那条走）；
  - 更长的 Disallow 前缀优先（robots 标准的 specificity 规则）；
  - `Allow` 可以放行 `Disallow` 覆盖的子树（长前缀优先）；
  - 显式 override 才允许访问，且必须能被审计（返回「绕过」事实而不是静默 True）。
"""

import unittest

from collector.crawl.robots import RobotsPolicy, RobotsDisallowed, parse_robots

OUR_UA = "ForkliftValuationCollector/1.0 (+contact: ops@hrwai.cn)"

WILDCARD_DENY_ALL = """
User-agent: Bytespider
Disallow: /

User-agent: *
Disallow: /
"""

PROVINCE_INDEX_ALLOWED = """
User-agent: *
Disallow: /Agency
Disallow: /Index

User-agent: ForkliftValuationCollector
Disallow: /Index
Allow: /statichtml
"""


class ParseRobotsTest(unittest.TestCase):
    def test_wildcard_deny_all_blocks_every_path(self):
        policy = parse_robots(WILDCARD_DENY_ALL, user_agent=OUR_UA)
        self.assertFalse(policy.allowed("https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/1.shtml"))
        self.assertFalse(policy.allowed("https://www.rmfysszc.gov.cn/"))

    def test_specific_agent_section_overrides_wildcard(self):
        policy = parse_robots(PROVINCE_INDEX_ALLOWED, user_agent=OUR_UA)
        # 我们自己的段只禁 /Index，并显式 Allow /statichtml。
        self.assertTrue(policy.allowed("https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/1.shtml"))
        self.assertFalse(policy.allowed("https://www1.rmfysszc.gov.cn/Index/tszlPage.aspx"))

    def test_longer_disallow_prefix_wins(self):
        text = """
User-agent: *
Disallow: /statichtml
Disallow: /statichtml/rm_xmdetail
"""
        policy = parse_robots(text, user_agent=OUR_UA)
        self.assertFalse(policy.allowed("https://x.test/statichtml/rm_xmdetail/1.shtml"))
        self.assertFalse(policy.allowed("https://x.test/statichtml/other.shtml"))

    def test_allow_beats_disallow_on_longer_prefix(self):
        text = """
User-agent: *
Disallow: /statichtml
Allow: /statichtml/rm_xmdetail
"""
        policy = parse_robots(text, user_agent=OUR_UA)
        self.assertTrue(policy.allowed("https://x.test/statichtml/rm_xmdetail/1.shtml"))
        self.assertFalse(policy.allowed("https://x.test/statichtml/other.shtml"))

    def test_empty_robots_allows_all(self):
        policy = parse_robots("", user_agent=OUR_UA)
        self.assertTrue(policy.allowed("https://x.test/anything"))

    def test_no_matching_agent_section_falls_back_to_wildcard(self):
        text = """
User-agent: SomeOtherBot
Disallow: /
"""
        policy = parse_robots(text, user_agent=OUR_UA)
        # 没有我们的段、也没有 * 段 ⇒ 无规则 ⇒ 允许。
        self.assertTrue(policy.allowed("https://x.test/anything"))


class RobotsPolicyTest(unittest.TestCase):
    def test_disallowed_path_raises_with_reason(self):
        policy = parse_robots(WILDCARD_DENY_ALL, user_agent=OUR_UA)
        with self.assertRaises(RobotsDisallowed) as ctx:
            policy.ensure_allowed("https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/1.shtml")
        self.assertIn("robots", str(ctx.exception).lower())

    def test_override_allows_but_reports_bypass(self):
        """override 不是静默 True：调用方必须能拿到「这次绕过了 robots」这个事实。"""
        policy = parse_robots(WILDCARD_DENY_ALL, user_agent=OUR_UA)
        decision = policy.decide(
            "https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/1.shtml",
            allow_override=True,
        )
        self.assertTrue(decision.allowed)
        self.assertTrue(decision.robots_disallowed)
        self.assertTrue(decision.overridden)

    def test_policy_bound_to_host_denies_other_host(self):
        """robots.txt 按 host 生效：拿 www 的策略放行 www1 是错的，必须 fail-closed。"""
        allow_all = "User-agent: *\nDisallow:\n"
        policy = parse_robots(allow_all, user_agent=OUR_UA, host="www.rmfysszc.gov.cn")
        self.assertTrue(policy.allowed("https://www.rmfysszc.gov.cn/statichtml/a.shtml"))
        self.assertFalse(
            policy.allowed("https://www1.rmfysszc.gov.cn/Agency/pmgg.aspx?fid=100"),
            "未核验的 host 必须按禁止处理（fail-closed）",
        )

    def test_unbound_policy_keeps_legacy_behavior(self):
        """host='' 表示不校验（兼容纯文本策略/单测注入），行为同之前。"""
        policy = parse_robots("User-agent: *\nDisallow:\n", user_agent=OUR_UA)
        self.assertTrue(policy.allowed("https://any.host.test/anything"))

    def test_normal_allow_is_not_marked_overridden(self):
        policy = parse_robots(PROVINCE_INDEX_ALLOWED, user_agent=OUR_UA)
        decision = policy.decide("https://www.rmfysszc.gov.cn/statichtml/rm_xmdetail/1.shtml")
        self.assertTrue(decision.allowed)
        self.assertFalse(decision.robots_disallowed)
        self.assertFalse(decision.overridden)


if __name__ == "__main__":
    unittest.main()
