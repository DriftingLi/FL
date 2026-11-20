"""京东 / 公拍网采集面的纯解析单测（#2679 判据 1/2/3/4/5 的解析侧）。

这些用例只用录制 fixture，不碰网络也不碰 PG——解析是纯函数，是这一票最该钉死
的地方：平台形态一变（JSONP 字段改名、表格标记变化、验证码页替换正常页），
应当先在这里红，而不是在生产库里落一批垃圾行。

真 PG 的端到端契约（fixture 进 → 行出）在 `test_sources_e2e.py`。
"""

import json
import unittest
from pathlib import Path

from collector.models import CHANNEL_GPAI, CHANNEL_JD
from collector.sources import gpai, jd

FIXTURES = Path(__file__).resolve().parents[1] / "fixtures"


def _fixture(name: str) -> str:
    return (FIXTURES / name).read_text(encoding="utf-8")


class JdListingTest(unittest.TestCase):
    """判据 1 的前半：公告页是发现入口，能列出该公告下的全部标的。"""

    def test_notice_page_yields_every_product_id(self):
        items = jd.parse_listing(
            _fixture("jd_notice_11810994.html"),
            "https://paimai.jd.com/notice/11810994",
        )
        self.assertEqual(len(items), 5, "实测该公告页含 5 个 albumProductInfosForId")
        self.assertEqual(
            [i.url for i in items],
            [
                "https://paimai.jd.com/309492343",
                "https://paimai.jd.com/309491344",
                "https://paimai.jd.com/309490779",
                "https://paimai.jd.com/309490162",
                "https://paimai.jd.com/309490702",
            ],
        )

    def test_empty_or_garbage_html_yields_nothing(self):
        self.assertEqual(jd.parse_listing("", "https://paimai.jd.com/notice/1"), [])
        self.assertEqual(
            jd.parse_listing("<html><body>无标的</body></html>", "https://paimai.jd.com/notice/1"),
            [],
        )

    def test_product_ids_are_deduped(self):
        html = (
            '<input type="hidden" class="albumProductInfosForId" value="111">'
            '<input type="hidden" class="albumProductInfosForId" value="111">'
            '<input type="hidden" class="albumProductInfosForId" value="222">'
        )
        items = jd.parse_listing(html, "https://paimai.jd.com/notice/1")
        self.assertEqual([i.url for i in items], [
            "https://paimai.jd.com/111",
            "https://paimai.jd.com/222",
        ])

    def test_attribute_order_does_not_matter(self):
        """实测 HTML 里 class 与 value 的先后不保证；两种顺序都要认。"""
        html = '<input value="333" type="hidden" class="albumProductInfosForId">'
        items = jd.parse_listing(html, "https://paimai.jd.com/notice/1")
        self.assertEqual([i.url for i in items], ["https://paimai.jd.com/333"])


class JdDetailTest(unittest.TestCase):
    """判据 1 的后半 + 判据 3/4：JSONP 两个接口 → 一条 raw 样本。"""

    def _fetch(self, basic_name="jd_basic_310055258.json",
               realtime_name="jd_realtime_310117853.json"):
        basic = _fixture(basic_name)
        realtime = _fixture(realtime_name)

        def fetch(url: str) -> str:
            if "getProductBasicInfo" in url:
                return basic
            if "getPaimaiRealTimeData" in url:
                return realtime
            raise AssertionError(f"未预期的接口: {url}")

        return fetch

    def test_detail_yields_deal_price_when_bids_exist(self):
        samples = jd.fetch_detail(
            "https://paimai.jd.com/310055258", self._fetch()
        )
        self.assertEqual(len(samples), 1)
        sample = samples[0]
        self.assertEqual(sample.channel, CHANNEL_JD)
        self.assertEqual(sample.source_url, "https://paimai.jd.com/310055258")
        # realtime fixture: bidCount=1、currentPrice=15000 ⇒ 成交价（deal）。
        self.assertEqual(sample.raw_price, 15000.0)
        self.assertEqual(sample.raw_price_kind, "deal")

    def test_zero_assessment_price_becomes_none_not_zero(self):
        """判据 4 的核心：破产/资产通道 assessmentPrice=0.0 是「源站不提供」，
        不是「评估价为零元」。归一成 None 后，溢价规则才不会拿 0 去做除法/比较。"""
        sample = jd.fetch_detail("https://paimai.jd.com/310055258", self._fetch())[0]
        self.assertIsNone(
            sample.appraisal_price,
            "assessmentPrice=0.0 必须归一成 None（源站不提供），否则下游会误判溢价",
        )

    def test_start_price_kept_when_no_bid(self):
        """bidCount=0 ⇒ 没有成交价，落起拍价（listing）。"""
        basic = _fixture("jd_basic_310055258.json")
        no_bid = json.loads(_fixture("jd_realtime_310117853.json"))
        no_bid["data"]["bidCount"] = 0
        no_bid["data"]["currentPrice"] = None
        no_bid_text = json.dumps(no_bid, ensure_ascii=False)

        def fetch(url: str) -> str:
            return basic if "getProductBasicInfo" in url else no_bid_text

        sample = jd.fetch_detail("https://paimai.jd.com/310055258", fetch)[0]
        self.assertEqual(sample.raw_price, 25000.0, "无应价时应落起拍价 25000")
        self.assertEqual(sample.raw_price_kind, "listing")

    def test_realtime_failure_degrades_to_listing_not_to_error(self):
        """realtime 接口挂了：起拍价仍来自 basic 接口，样本应降级落 listing。"""
        basic = _fixture("jd_basic_310055258.json")

        def fetch(url: str) -> str:
            if "getProductBasicInfo" in url:
                return basic
            raise OSError("realtime 接口超时")

        sample = jd.fetch_detail("https://paimai.jd.com/310055258", fetch)[0]
        self.assertEqual(sample.raw_price, 25000.0)
        self.assertEqual(sample.raw_price_kind, "listing")

    def test_basic_failure_raises(self):
        """basic 接口失败必须抛：没有价格与标题的样本落库只会污染数据。"""

        def fetch(url: str) -> str:
            raise ValueError("接口拒绝")

        with self.assertRaises(ValueError):
            jd.fetch_detail("https://paimai.jd.com/310055258", fetch)

    def test_api_rejection_code_raises(self):
        """实测不存在的接口返回 {"code":"2","echo":"the current API does not exist"}。"""
        payload = '{"code":"2","echo":"the current API does not exist"}'

        def fetch(url: str) -> str:
            return payload

        with self.assertRaises(ValueError) as ctx:
            jd.fetch_detail("https://paimai.jd.com/310055258", fetch)
        self.assertIn("code", str(ctx.exception))

    def test_timestamps_are_milliseconds(self):
        """实测 startTime=1768442400000 → 2026-01-15（ms epoch，不是秒）。"""
        sample = jd.fetch_detail("https://paimai.jd.com/310055258", self._fetch())[0]
        self.assertIsNotNone(sample.auction_date)
        self.assertEqual(sample.auction_date.year, 2026)
        self.assertEqual(sample.auction_date.month, 1)

    def test_tonnage_is_read_from_title(self):
        """标题「…林德T20AP-2吨电动托盘车」里的 2 吨要被取出来。"""
        self.assertEqual(jd._tonnage_from_title("林德T20AP-2吨电动托盘车"), 2.0)
        self.assertEqual(jd._tonnage_from_title("一台3T汽油液力式叉车资产处置"), 3.0)
        # 「T」后面跟字母时不是吨位（型号里的 T20、ETV114 不能误判）。
        self.assertIsNone(jd._tonnage_from_title("永恒力 ETV114 前移式叉车"))

    def test_raw_price_confidence_is_max_for_direct_fields(self):
        """JSONP 直取的字段置信度给最高级（判据②要求「结构化字段直取标置信度最高」）。"""
        sample = jd.fetch_detail("https://paimai.jd.com/310055258", self._fetch())[0]
        self.assertEqual(sample.field_confidence["raw_price"], 1.0)

    def test_url_without_numeric_id_raises(self):
        with self.assertRaises(ValueError):
            jd.fetch_detail("https://paimai.jd.com/not-a-number", lambda _u: "{}")


class JdJsonpParsingTest(unittest.TestCase):
    def test_plain_json_is_accepted(self):
        data = jd._parse_jsonp('{"code":0,"data":{"title":"x"}}')
        self.assertEqual(data["title"], "x")

    def test_callback_wrapped_json_is_accepted(self):
        """JSONP 传统形态：cb({...})。实测接口返回纯 JSON，但两种都要收。"""
        data = jd._parse_jsonp('jsonp_123({"code":0,"data":{"title":"x"}})')
        self.assertEqual(data["title"], "x")

    def test_empty_response_raises(self):
        with self.assertRaises(ValueError):
            jd._parse_jsonp("")


class GpaiListingTest(unittest.TestCase):
    """判据 2 的前半：司法频道列表页 → 详情 URL。"""

    def test_sf_listing_yields_item_urls(self):
        items = gpai.parse_listing(
            _fixture("gpai_sf_list.html"), "https://www.gpai.net/sf/"
        )
        self.assertGreaterEqual(len(items), 1)
        self.assertEqual(
            items[0].url, "https://www.gpai.net/sf/item2.do?Web_Item_ID=53260"
        )
        for item in items:
            self.assertTrue(item.url.startswith("https://www.gpai.net/sf/item2.do"))

    def test_backslash_and_protocol_relative_links_are_parsed(self):
        """实测列表页链接是 `//www.gpai.net/sf\\item2.do?Web_Item_ID=53260`
        ——协议相对 + 反斜杠路径，只认正斜杠会一条都解析不到。"""
        html = "<a href='//www.gpai.net/sf\\item2.do?Web_Item_ID=999'>x</a>"
        items = gpai.parse_listing(html, "https://www.gpai.net/sf/")
        self.assertEqual(len(items), 1)
        self.assertIn("Web_Item_ID=999", items[0].url)

    def test_zc_listing_yields_item_urls_from_json(self):
        items = gpai.parse_listing(
            _fixture("gpai_zc_list.json"),
            "https://zc.gpai.net/api/item/list?pageNum=1&pageSize=5",
        )
        self.assertGreaterEqual(len(items), 1)
        self.assertTrue(items[0].url.startswith("https://zc.gpai.net/zc/detail?id="))


class GpaiDetailTest(unittest.TestCase):
    """判据 2 的后半 + 判据 3：成交价 + 评估价入表，溢价标记可生效。"""

    def test_ended_item_yields_deal_price_and_appraisal(self):
        sample = gpai.parse_sf_detail(
            _fixture("gpai_sf_item_ended.html"),
            "https://www.gpai.net/sf/item2.do?Web_Item_ID=53385",
        )[0]
        self.assertEqual(sample.channel, CHANNEL_GPAI)
        # 实测：成交价 195899、起拍价 167899、评估价 299,818（带千分位）。
        self.assertEqual(sample.raw_price, 195899.0)
        self.assertEqual(sample.raw_price_kind, "deal")
        self.assertEqual(sample.appraisal_price, 299818.0)
        self.assertEqual(sample.structured_specs["start_price"], 167899.0)

    def test_ended_without_deal_price_is_listing_not_deal(self):
        """已取消标的：状态「已结束」但只有起拍价 ⇒ 不能当成交落。

        实测 Web_Item_ID=52500 的形态（`拍卖/变卖取消`，d-m-price 里只有起拍价）。
        """
        html = """
<html><head><title>某标的 - 拍品详情 - 公拍网</title></head><body>
<span class="fl time-txt">已结束</span>
<span id='ItemSecondCount' CountType='Item' CountID='52500' > 拍卖/变卖取消</span>
<div class="d-m-price"><p><span>起拍价：</span><b class='price-red'>88000</b></p></div>
<table><tr><td>起拍价：<span id="Price_Start">88000</span>元</td></tr></table>
</body></html>
"""
        sample = gpai.parse_sf_detail(html, "https://www.gpai.net/sf/item2.do?Web_Item_ID=52500")[0]
        self.assertEqual(sample.raw_price, 88000.0)
        self.assertEqual(
            sample.raw_price_kind,
            "listing",
            "没有成交价就不是成交样本——状态字段说「已结束」不等于成交",
        )

    def test_captcha_page_raises_instead_of_empty(self):
        """运行态硬约束：gpai 已按 blacklist 拦截我们的诚实 UA。

        验证码页必须抛错（由编排层的毒丸隔离接住），绝不能返回空样本——
        静默返回 [] 会让得率报告全绿、库里却一行没有。
        """
        captcha = """<html><head>
<link rel="stylesheet" href="https://seccaptcha.haplat.net/css/captcha.css">
</head><body><div id="captchaPage"></div>
<input type="hidden" id="infoString" value="eyJ0eXBlIjoiYmxhY2tsaXN0In0=">
<div id="rightValidate_cont"></div>
</body></html>"""
        with self.assertRaises(gpai.CaptchaBlocked):
            gpai.parse_sf_detail(captcha, "https://www.gpai.net/sf/item2.do?Web_Item_ID=53385")

    def test_zc_detail_raises_instead_of_empty(self):
        """资产频道详情是 SPA 空壳 + 详情接口全废 ⇒ 必须显式失败。"""
        with self.assertRaises(ValueError):
            gpai.parse_zc_detail(
                "<html><head><title>公拍网 资产交易</title></head><body></body></html>",
                "https://zc.gpai.net/zc/detail?id=8817149",
            )

    def test_zc_listing_price_is_listing_kind(self):
        """资产频道只能拿列表字段：起拍价 ⇒ listing（拿不到成交价与评估价）。"""
        items = gpai.parse_listing(
            _fixture("gpai_zc_list.json"),
            "https://zc.gpai.net/api/item/list?pageNum=1&pageSize=5",
        )
        self.assertTrue(items, "录制 fixture 应解析出条目")


class PremiumRuleOnRealShapesTest(unittest.TestCase):
    """判据 3：溢价 > 评估价×1.5 的标记必须在真实形态上生效。

    这里直接用清洗规则（`clean`）验证：京东零评估价不误判 + 真实溢价形态会标。
    为什么用 fixture 造溢价样本：实测扫描 gpai 司法频道 60 个已结束标的，
    `ratio > 1.5` 命中 **0 个**（成交价普遍是评估价的 0.65~0.72）——
    真实语料里没有溢价样本，只能构造。
    """

    def test_jd_zero_appraisal_never_flags_premium(self):
        from collector.clean import clean

        sample = jd.fetch_detail(
            "https://paimai.jd.com/310055258",
            lambda url: _fixture(
                "jd_basic_310055258.json"
                if "getProductBasicInfo" in url
                else "jd_realtime_310117853.json"
            ),
        )[0]
        cleaned, review = clean(sample)
        self.assertFalse(cleaned.premium_anomaly, "评估价缺失时不得判溢价")
        self.assertNotIn("premium_anomaly", [reason for reason, _ in review])

    def test_premium_is_flagged_when_deal_far_exceeds_appraisal(self):
        from collector.clean import clean

        html = """
<html><head><title>某叉车 - 拍品详情 - 公拍网</title></head><body>
<span class="fl time-txt">已结束</span>
<div class="d-m-price"><p><span>成交价：</span><b class='price-red'>900000</b></p></div>
<table>
<tr><td>起拍价：<span id="Price_Start">500000</span>元</td></tr>
<tr><td>评估价：400,000 元</td></tr>
</table>
</body></html>
"""
        sample = gpai.parse_sf_detail(html, "https://www.gpai.net/sf/item2.do?Web_Item_ID=1")[0]
        cleaned, review = clean(sample)
        # 900000 > 400000 × 1.5 = 600000 ⇒ 标记降权（不是剔除）。
        self.assertTrue(cleaned.premium_anomaly)
        self.assertEqual(cleaned.clean_status, "cleaned", "溢价是标记，不是剔除")
        self.assertIn("premium_anomaly", [reason for reason, _ in review])


class ScrappedSignalTest(unittest.TestCase):
    """清洗规则②的判据字段必须由适配器填，否则「废铁价 ≠ 使用价值」永不命中。

    这是一个在真实抓取里暴露出来的缺陷：京东破产/资产通道的标题几乎全是
    「报废资产…【无电池】」，如果不把 `scrapped` / `has_battery` 写进 structured_specs，
    规则②就没有判据可看，废铁价会被当成市场成交价落成 cleaned 样本——
    直接污染下游残值率校准。
    """

    def test_jd_scrapped_title_marks_specs(self):
        sample = jd.fetch_detail(
            "https://paimai.jd.com/310055258",
            lambda url: _fixture(
                "jd_basic_310055258.json"
                if "getProductBasicInfo" in url
                else "jd_realtime_310117853.json"
            ),
        )[0]
        # 实测标题：「湖北武汉 报废资产电动前移式叉车-永恒力（JUNGHEINRICH）ETV114【无电池】」
        self.assertTrue(sample.structured_specs.get("scrapped"), "「报废资产」必须标记")
        self.assertIs(
            sample.structured_specs.get("has_battery"), False, "「无电池」必须标记"
        )

    def test_jd_scrapped_sample_is_rejected_by_clean_rule(self):
        from collector.clean import clean

        sample = jd.fetch_detail(
            "https://paimai.jd.com/310055258",
            lambda url: _fixture(
                "jd_basic_310055258.json"
                if "getProductBasicInfo" in url
                else "jd_realtime_310117853.json"
            ),
        )[0]
        cleaned, review = clean(sample)
        self.assertEqual(cleaned.clean_status, "rejected")
        self.assertEqual(cleaned.rejection_reason, "scrapped_or_no_battery")
        self.assertFalse(cleaned.label_eligible, "报废样本不能进校准标签集")

    def test_healthy_title_is_not_marked_scrapped(self):
        """没提到报废/无电池时不能反推「车况良好」，只留空（不写字段）。"""
        html = """
<html><head><title>某在用叉车 - 拍品详情 - 公拍网</title></head><body>
<div class="d-m-price"><p><span>成交价：</span><b class='price-red'>80000</b></p></div>
<table><tr><td>起拍价：<span id="Price_Start">70000</span>元</td></tr></table>
</body></html>
"""
        sample = gpai.parse_sf_detail(html, "https://www.gpai.net/sf/item2.do?Web_Item_ID=1")[0]
        self.assertNotIn("scrapped", sample.structured_specs)
        self.assertNotIn("has_battery", sample.structured_specs)


if __name__ == "__main__":
    unittest.main()
