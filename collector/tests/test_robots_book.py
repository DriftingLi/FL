"""多 host 采集面的 robots 分派单测（#2679 判据 5 的合规侧）。

为什么单列一个文件：多 host 是这一票**唯一**的合规新风险面。京东的列表页、
标的页、JSONP 接口分处三个 host，`RobotsPolicy` 按 host 绑定、遇到别的 host 一律
fail-closed——拿它直接抓第二个 host 会把采集面判死。`RobotsBook` 就是为此而生，
它必须满足三条：

  1. 每个 host 各用自己的 robots（不串味）；
  2. 未核验的 host 仍 fail-closed（不能因为「没读」就放行）；
  3. 软 200 的错误页不能被当成「站点允许一切」（实测三个平台都有这个陷阱）。
"""

import unittest

from collector.crawl.client import _looks_like_robots, load_robots, load_robots_book
from collector.crawl.robots import RobotsDisallowed, RobotsBook, parse_robots

UA = "ForkliftValuationCollector/1.0"

JD_ITEM = "https://paimai.jd.com/310055258"
JD_API = "https://api.m.jd.com/?appid=paimai&functionId=getProductBasicInfo"
JD_WWW = "https://www.jd.com/robots.txt"


def _book(*policies):
    return RobotsBook(tuple(policies))


class RobotsBookDispatchTest(unittest.TestCase):
    def test_each_host_uses_its_own_robots(self):
        book = _book(
            parse_robots("User-agent: *\nDisallow: /\n", user_agent=UA, host="paimai.jd.com"),
            parse_robots("User-agent: *\nDisallow:\n", user_agent=UA, host="api.m.jd.com"),
        )
        # paimai 全禁、api 全放：两个 host 不能串味。
        self.assertFalse(book.allowed(JD_ITEM))
        self.assertTrue(book.allowed(JD_API))

    def test_unknown_host_is_fail_closed(self):
        book = _book(
            parse_robots("User-agent: *\nDisallow:\n", user_agent=UA, host="paimai.jd.com")
        )
        self.assertFalse(
            book.allowed(JD_API),
            "未核验 host 必须按禁止处理——「没读过」不等于「没有限制」",
        )

    def test_unknown_host_can_be_overridden_with_audit(self):
        book = _book(
            parse_robots("User-agent: *\nDisallow:\n", user_agent=UA, host="paimai.jd.com")
        )
        decision = book.decide(JD_API, allow_override=True)
        self.assertTrue(decision.allowed)
        self.assertTrue(
            decision.overridden,
            "人工放行未核验 host 也要记 overridden，审计必须能看见",
        )

    def test_disallowed_host_raises_with_shared_message(self):
        book = _book(
            parse_robots("User-agent: *\nDisallow: /\n", user_agent=UA, host="paimai.jd.com")
        )
        with self.assertRaises(RobotsDisallowed) as ctx:
            book.ensure_allowed(JD_ITEM)
        self.assertIn("--allow-robots-override", str(ctx.exception))

    def test_policy_without_host_is_rejected(self):
        with self.assertRaises(ValueError):
            RobotsBook((parse_robots("User-agent: *\nDisallow:\n", user_agent=UA),))

    def test_hosts_property_lists_verified_hosts(self):
        book = _book(
            parse_robots("User-agent: *\nDisallow:\n", user_agent=UA, host="b.example"),
            parse_robots("User-agent: *\nDisallow:\n", user_agent=UA, host="a.example"),
        )
        self.assertEqual(book.hosts, ("a.example", "b.example"))


class LoadRobotsBookTest(unittest.TestCase):
    def test_reads_each_origin_independently(self):
        """一个 host 读不到 robots 只让**那个 host** 需放行，不连坐其它 host。"""
        seen: list[str] = []

        def fetcher(url: str) -> str:
            seen.append(url)
            if url.startswith("https://paimai.jd.com"):
                raise OSError("paimai robots 拿不到")
            return "User-agent: *\nDisallow:\n"

        book = load_robots_book(
            origins=("https://paimai.jd.com", "https://api.m.jd.com"),
            fetcher=fetcher,
            user_agent_string=UA,
        )
        self.assertEqual(
            sorted(seen),
            [
                "https://api.m.jd.com/robots.txt",
                "https://paimai.jd.com/robots.txt",
            ],
        )
        self.assertFalse(book.allowed(JD_ITEM), "读不到 robots 的 host 必须 fail-closed")
        self.assertTrue(book.allowed(JD_API), "另一个 host 不受连坐")


class SoftTwoHundredTrapTest(unittest.TestCase):
    """实测三个平台都有「robots.txt 返回 200 但不是 robots」的软 200 陷阱。

    如果不识别，空文本会被解析成「无规则 ⇒ 全部允许」——把「站点没给我们 robots」
    误判成「站点允许一切」，是合规上的反向错误。
    """

    def test_error_page_is_not_robots(self):
        self.assertFalse(
            _looks_like_robots(
                "<!DOCTYPE html><html><head><title>您所访问的页面不存在</title></head></html>"
            )
        )

    def test_json_error_body_is_not_robots(self):
        # 实测 api.m.jd.com/robots.txt 返回这个。
        self.assertFalse(
            _looks_like_robots(
                '{"code":"1","echo":"required string parameter \'functionId\' is not present"}'
            )
        )

    def test_spa_shell_is_not_robots(self):
        # 实测 zc.gpai.net/robots.txt 返回 SPA 首页。
        self.assertFalse(_looks_like_robots("<html><body><div id=app></div></body></html>"))

    def test_empty_body_is_not_robots(self):
        self.assertFalse(_looks_like_robots("   \n  "))

    def test_real_robots_is_recognised(self):
        self.assertTrue(_looks_like_robots("User-agent: *\nDisallow: /sf/search.do\n"))
        self.assertTrue(_looks_like_robots("# comment\nUser-agent: *\nDisallow:\n"))
        self.assertTrue(_looks_like_robots("Sitemap: https://example.com/sitemap.xml\n"))

    def test_soft_two_hundred_fails_closed_through_load_robots(self):
        def fetcher(url: str) -> str:
            return "<!DOCTYPE html><html><body>404 页面</body></html>"

        policy = load_robots(base_url="https://paimai.jd.com", fetcher=fetcher, user_agent_string=UA)
        self.assertFalse(
            policy.allowed(JD_ITEM),
            "软 200 错误页必须 fail-closed，不能解析成「允许全部」",
        )


class RealPlatformRobotsTest(unittest.TestCase):
    """用实测到的真实 robots 文本钉住判定结果（不联网，文本来自现场抓取）。"""

    GPAI_ROBOTS = """User-agent:*
Disallow:/upload/
Disallow:/tb/search.do
Disallow:/tb/Search.do
Disallow:/api/
Disallow:/bm/Search.do
Disallow:/bm/
Disallow:/sf/search.do
Disallow:/sf/Search.do
Disallow:/sf/noticeDetail.do

User-agent: SemrushBot
Disallow: /
"""

    def test_gpai_sf_listing_and_detail_are_allowed(self):
        policy = parse_robots(self.GPAI_ROBOTS, user_agent=UA, host="www.gpai.net")
        self.assertTrue(
            policy.allowed("https://www.gpai.net/sf/"),
            "/sf/ 不在 Disallow 列表里 ⇒ 司法频道列表页可采",
        )
        self.assertTrue(
            policy.allowed("https://www.gpai.net/sf/item2.do?Web_Item_ID=53260"),
            "/sf/item2.do 不在 Disallow 列表里 ⇒ 详情页可采",
        )

    def test_gpai_ended_listing_is_disallowed(self):
        policy = parse_robots(self.GPAI_ROBOTS, user_agent=UA, host="www.gpai.net")
        self.assertFalse(
            policy.allowed("https://www.gpai.net/sf/search.do?action=court&restate=3"),
            "/sf/search.do 被明示禁止 ⇒ 已结束列表必须走 --allow-robots-override",
        )

    def test_gpai_asset_channel_api_is_disallowed(self):
        policy = parse_robots(self.GPAI_ROBOTS, user_agent=UA, host="www.gpai.net")
        self.assertFalse(
            policy.allowed("https://www.gpai.net/api/item/list"),
            "/api/ 被明示禁止",
        )


if __name__ == "__main__":
    unittest.main()
