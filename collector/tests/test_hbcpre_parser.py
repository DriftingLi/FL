"""湖北产权交易所省镜像采集面的解析单测（#2680 判据 1/2/3）。

只用录制 fixture、不碰网络也不碰 PG。这里钉的是三个最容易错的地方：

  ① 列表行缺 `</tr>`（实测源站如此）——按表格结构分块会漏一半条目；
  ② 单位陷阱——挂牌详情的标签是「挂牌价格（万元）」，成交详情是「（元）」，
     差 10000 倍会让残值率标签全错；
  ③ 租赁权陷阱——当期 9 条成交公告全是「三年承租权」，成交金额是三年租金总额、
     评估价是年租金评估价，量纲与设备残值无关。

真 PG 的端到端契约（fixture 进 → 行出、两价齐备）在 `test_hbcpre_e2e.py`。
"""

import unittest
from pathlib import Path

from collector.models import CHANNEL_HBCPRE
from collector.sources import hbcpre

FIXTURES = Path(__file__).resolve().parents[1] / "fixtures"


def _fixture(name: str) -> str:
    return (FIXTURES / name).read_text(encoding="utf-8")


JYGG_LIST = "https://www.hbggzyfwpt.cn/jyxx/whcq/jygg"
JYJG_LIST = "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjg"


class ListingTest(unittest.TestCase):
    def test_jygg_listing_yields_detail_urls(self):
        items = hbcpre.parse_listing(_fixture("hbcpre_jygg_list.html"), JYGG_LIST)
        self.assertEqual(len(items), 2, "录制 fixture 里有两个挂牌条目")
        self.assertEqual(
            items[0].url,
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=e3484a1f-a37d-40dd-844e-fc1eab7c87d7",
        )
        for item in items:
            self.assertTrue(item.url.startswith("https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail"))

    def test_jyjg_listing_yields_detail_urls(self):
        items = hbcpre.parse_listing(_fixture("hbcpre_jyjg_list.html"), JYJG_LIST)
        self.assertEqual(len(items), 2)
        self.assertIn("jyjgDetail", items[0].url)

    def test_title_attribute_is_captured(self):
        items = hbcpre.parse_listing(_fixture("hbcpre_jygg_list.html"), JYGG_LIST)
        self.assertIn("承租权", items[0].title)

    def test_missing_tr_close_does_not_lose_entries(self):
        """实测源站列表行缺 `</tr>`：`<td>` 结束直接接下一个 `<tr>`。

        按表格结构分块（等 `</tr>`）会漏掉后半部分条目——所以解析器按 `<a>` 取链接。
        """
        html = """
<table><tr><td><a href="/jyxx/whcq/jyggDetail?guid=aaa" title="第一条">一</a></td>
<td>2026-10-30</td>
<tr><td><a href="/jyxx/whcq/jyggDetail?guid=bbb" title="第二条">二</a></td>
<td>2026-10-29</td>
</table>
"""
        items = hbcpre.parse_listing(html, JYGG_LIST)
        self.assertEqual(len(items), 2, "缺 </tr> 也不能漏条目")
        self.assertIn("guid=aaa", items[0].url)
        self.assertIn("guid=bbb", items[1].url)

    def test_relative_links_become_absolute(self):
        """幂等判重的键是绝对地址，相对链接必须补全。"""
        html = '<a href="/jyxx/whcq/jyggDetail?guid=x" title="t">x</a>'
        items = hbcpre.parse_listing(html, JYGG_LIST)
        self.assertEqual(
            items[0].url, "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=x"
        )

    def test_empty_html_yields_nothing(self):
        self.assertEqual(hbcpre.parse_listing("", JYGG_LIST), [])
        self.assertEqual(hbcpre.parse_listing("<html></html>", JYGG_LIST), [])


class JyggDetailTest(unittest.TestCase):
    """挂牌详情：4-td 双栏表格、标签带半角冒号、价格单位是万元。"""

    URL = "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=7ca4b837-999f-432d-8d02-6f4426fab5ea"

    def _sample(self):
        return hbcpre.fetch_detail(self.URL, lambda _u: _fixture("hbcpre_jygg_detail.html"))[0]

    def test_listing_price_wans_converted_to_yuan(self):
        """判据①的核心陷阱：`挂牌价格（万元）: 2.43` ⇒ 24300 元，不是 2.43 元。"""
        sample = self._sample()
        self.assertEqual(
            sample.raw_price, 24300.0,
            "标签写明（万元）就必须乘 10000；差 10000 倍会让残值率标签全错",
        )
        self.assertEqual(sample.raw_price_kind, "listing")

    def test_listing_price_column_mirrors_raw_price(self):
        """挂牌样本上两个价格列同值（列语义恒定：listing_price 就是挂牌价）。"""
        sample = self._sample()
        self.assertEqual(sample.listing_price, sample.raw_price)

    def test_no_appraisal_on_listing_page(self):
        """挂牌页没有评估价——那是成交页才有的字段。"""
        self.assertIsNone(self._sample().appraisal_price)

    def test_structured_fields_are_extracted(self):
        sample = self._sample()
        self.assertEqual(sample.channel, CHANNEL_HBCPRE)
        self.assertEqual(sample.structured_specs["project_no"], "202610092")
        self.assertEqual(sample.structured_specs["tonnage"], 3.0)
        self.assertIn("叉车", sample.structured_specs["title"])
        self.assertTrue(sample.structured_specs.get("equipment_relevant"))

    def test_publish_date_is_parsed(self):
        sample = self._sample()
        self.assertIsNotNone(sample.auction_date)
        self.assertEqual(sample.auction_date.year, 2026)
        self.assertEqual(sample.auction_date.month, 4)

    def test_price_confidence_is_max_for_direct_fields(self):
        """判据①：结构化字段直取 ⇒ 置信度最高级。"""
        sample = self._sample()
        self.assertEqual(sample.field_confidence["raw_price"], 1.0)


class JyjgDetailTest(unittest.TestCase):
    """成交详情：2-td 单栏表格、标签不带冒号、两个价都在同一页。"""

    URL = "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=6cbdd495-fe4c-4947-b039-d83e5d365a99"

    def _sample(self):
        return hbcpre.fetch_detail(self.URL, lambda _u: _fixture("hbcpre_jyjg_detail.html"))[0]

    def test_both_prices_present_on_same_sample(self):
        """判据②：同一样本两价齐备（挂牌价 + 成交金额）。

        实测：评估价 199202 / 挂牌价 199300 / 成交金额 268200。
        """
        sample = self._sample()
        self.assertEqual(sample.raw_price, 268200.0, "raw_price 是成交金额")
        self.assertEqual(sample.raw_price_kind, "deal")
        self.assertEqual(sample.listing_price, 199300.0, "listing_price 是挂牌价")
        self.assertIsNotNone(sample.listing_price, "两价齐备是判据②的硬要求")

    def test_appraisal_price_is_captured(self):
        sample = self._sample()
        self.assertEqual(sample.appraisal_price, 199202.0)

    def test_yuan_unit_is_not_multiplied(self):
        """成交页标签写的是（元），不能再乘 10000。"""
        sample = self._sample()
        self.assertLess(sample.raw_price, 1000000, "成交金额是元，不是万元")

    def test_deal_below_1000x_listing(self):
        """合理性护栏：成交/挂牌 应在 1 附近（这个样本 1.345），不是 10000 倍。"""
        sample = self._sample()
        ratio = sample.raw_price / sample.listing_price
        self.assertGreater(ratio, 0.1)
        self.assertLess(ratio, 10.0)

    def test_project_no_is_captured_for_cross_page_pairing(self):
        """项目编号是挂牌详情与成交详情跨页关联的键（两个页面 guid 不同）。"""
        self.assertEqual(self._sample().structured_specs["project_no"], "202510082")

    def test_title_without_colon_label_is_parsed(self):
        """成交页标签不带冒号（`<td>挂牌价格（元）</td>`）——靠已知标签名清单识别。"""
        sample = self._sample()
        self.assertEqual(sample.listing_price, 199300.0)


class LeaseRightTest(unittest.TestCase):
    """租赁权/承租权：量纲与设备残值无关，必须剔除而不是当设备样本落库。"""

    def test_lease_title_is_marked(self):
        html = _fixture("hbcpre_jyjg_detail.html").replace(
            "992只废旧蓄电池处置成交公告",
            "宜昌市夷兴大道13-32档二楼B区三年承租权成交公告",
        )
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=y", lambda _u: html
        )[0]
        self.assertTrue(sample.structured_specs.get("lease_right"))

    def test_lease_sample_is_rejected_by_clean_rule(self):
        """实测当期 9 条成交公告全是三年承租权：成交/评估 3.86~7.33 是量纲差不是溢价。"""
        from collector.clean import clean

        html = _fixture("hbcpre_jyjg_detail.html").replace(
            "992只废旧蓄电池处置成交公告",
            "宜昌市夷兴大道13-32档二楼B区三年承租权成交公告",
        )
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=y", lambda _u: html
        )[0]
        cleaned, review = clean(sample)
        self.assertEqual(cleaned.clean_status, "rejected")
        self.assertEqual(cleaned.rejection_reason, "lease_right_not_equipment")
        self.assertFalse(cleaned.label_eligible, "租赁权样本不得有标签资格")
        self.assertIn("lease_right_not_equipment", [r for r, _ in review])

    def test_equipment_sample_is_not_rejected_as_lease(self):
        from collector.clean import clean

        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=x",
            lambda _u: _fixture("hbcpre_jyjg_detail.html"),
        )[0]
        cleaned, _ = clean(sample)
        self.assertNotEqual(cleaned.rejection_reason, "lease_right_not_equipment")

    def test_lease_rule_runs_before_placeholder_rule(self):
        """租赁样本必须先按量纲剔除，否则会被占位价规则误判成 placeholder_price。

        三年租金 / 吨位 可能低于 1000 元/吨的阈值——原因会归错，得率报告读起来
        像「价格异常」而不是「这不是设备样本」。
        """
        from collector.clean import clean

        # 造一条「2 吨、成交价 1000 元」的租赁样本：1000 < 2×1000 会踩占位价线。
        html = f"""
<html><body>
<div class="detailTitle">某商铺三年承租权成交公告</div>
<div class="detailNeirong"><div class="table table_cj"><table>
<tr><td>项目编号</td><td>202600001</td></tr>
<tr><td>挂牌价格（元）</td><td>900</td></tr>
<tr><td>成交金额（元）</td><td>1000</td></tr>
</table></div></div>
</body></html>
"""
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=z", lambda _u: html
        )[0]
        cleaned, _ = clean(sample)
        self.assertEqual(
            cleaned.rejection_reason,
            "lease_right_not_equipment",
            "租赁权必须先于占位价判定，否则原因归错",
        )


class NoEquipmentLeakTest(unittest.TestCase):
    """这个源是通用产权平台，非叉车条目（房产/蓄电池/租赁权）不能产出设备标签。"""

    def test_non_equipment_sample_is_not_marked_equipment_relevant(self):
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=x",
            lambda _u: _fixture("hbcpre_jyjg_detail.html"),
        )[0]
        self.assertNotIn("equipment_relevant", sample.structured_specs)

    def test_property_listing_is_rejected_as_not_equipment(self):
        """真实抓取暴露的缺陷：房产样本以 22,860,830 元落成 cleaned + label_eligible。

        它永远匹配不上叉车原价、永远算不出残值率，只会以 (NULL,NULL,NULL) 组
        堆进 market_unmatched_queue 顶部，把人工补原价的注意力引到房产上。
        """
        from collector.clean import clean

        html = """
<html><body>
<div class="detailTitle">北京市海淀区远大园五区1号楼2层七单元2F号房产处置</div>
<div class="detailNeirong"><table>
<tr><td width="20%">项目编号:</td><td>202610219</td></tr>
<tr><td width="20%">挂牌价格（万元）:</td><td>2286.083</td></tr>
</table></div>
</body></html>
"""
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=p", lambda _u: html
        )[0]
        cleaned, review = clean(sample)
        self.assertEqual(cleaned.clean_status, "rejected")
        self.assertEqual(cleaned.rejection_reason, "not_equipment_sample")
        self.assertFalse(cleaned.label_eligible, "非设备样本不得有标签资格")
        self.assertIn("not_equipment_sample", [r for r, _ in review])

    def test_bulk_assets_package_is_rejected_as_not_equipment(self):
        """「一批废旧资产处置」这类无规格物资包：没有品牌/吨位，也匹配不上原价。"""
        from collector.clean import clean

        html = """
<html><body>
<div class="detailTitle">一批废旧资产处置</div>
<div class="detailNeirong"><table>
<tr><td width="20%">项目编号:</td><td>202610217</td></tr>
<tr><td width="20%">挂牌价格（万元）:</td><td>0.1</td></tr>
</table></div>
</body></html>
"""
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=b", lambda _u: html
        )[0]
        cleaned, _ = clean(sample)
        self.assertEqual(cleaned.rejection_reason, "not_equipment_sample")

    def test_forklift_with_tonnage_is_not_rejected(self):
        """真设备样本必须放行：有吨位 ⇒ 有匹配价值，不能误杀。"""
        from collector.clean import clean

        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=x",
            lambda _u: _fixture("hbcpre_jygg_detail.html"),
        )[0]
        cleaned, _ = clean(sample)
        self.assertNotEqual(cleaned.rejection_reason, "not_equipment_sample")
        self.assertEqual(cleaned.clean_status, "cleaned")

    def test_equipment_title_without_tonnage_is_not_rejected(self):
        """标题提到叉车但没抽到吨位：仍属本领域，不能因缺规格就剔。"""
        from collector.clean import clean

        html = """
<html><body>
<div class="detailTitle">某单位叉车一台处置</div>
<div class="detailNeirong"><table>
<tr><td width="20%">项目编号:</td><td>202610300</td></tr>
<tr><td width="20%">挂牌价格（万元）:</td><td>3.5</td></tr>
</table></div>
</body></html>
"""
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=t", lambda _u: html
        )[0]
        cleaned, _ = clean(sample)
        self.assertNotEqual(cleaned.rejection_reason, "not_equipment_sample")

    def test_not_equipment_rule_applies_to_all_sources(self):
        """这条规则不依赖适配器写标记——对所有采集面按文本判，避免新适配器漏写就放行。

        （第一版实现只看 `equipment_relevant` 标记，结果误伤了 #2679 的公拍网
        溢价样本——适配器没写这个标记。规则必须自己看文本。）
        """
        from collector.clean import clean
        from collector.models import CHANNEL_GPAI, RawSample

        sample = RawSample(
            channel=CHANNEL_GPAI,
            source_url="https://www.gpai.net/sf/item2.do?Web_Item_ID=1",
            raw_description={"title": "某市某区某路100号商铺", "body": "商铺一间"},
            raw_price=900000.0,
            raw_price_kind="deal",
            appraisal_price=400000.0,
            structured_specs={},
            field_confidence={"raw_price": 1.0},
        )
        cleaned, _ = clean(sample)
        self.assertEqual(
            cleaned.rejection_reason,
            "not_equipment_sample",
            "没有任何采集面的标记也要能识别非设备样本",
        )


class DispatchTest(unittest.TestCase):
    def test_unknown_detail_url_raises(self):
        with self.assertRaises(ValueError):
            hbcpre.fetch_detail(
                "https://www.hbggzyfwpt.cn/jyxx/whcq/otherDetail?guid=x", lambda _u: "<html></html>"
            )

    def test_jygg_url_routes_to_listing_parser(self):
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyggDetail?guid=x",
            lambda _u: _fixture("hbcpre_jygg_detail.html"),
        )[0]
        self.assertEqual(sample.raw_price_kind, "listing")

    def test_jyjg_url_routes_to_deal_parser(self):
        sample = hbcpre.fetch_detail(
            "https://www.hbggzyfwpt.cn/jyxx/whcq/jyjgDetail?guid=x",
            lambda _u: _fixture("hbcpre_jyjg_detail.html"),
        )[0]
        self.assertEqual(sample.raw_price_kind, "deal")


class SpecTest(unittest.TestCase):
    def test_spec_is_structured_and_keeps_undated(self):
        spec = hbcpre.SPEC
        self.assertTrue(spec.structured, "字段来自服务端表格 ⇒ 跳过 LLM")
        self.assertTrue(spec.keep_undated, "列表页不给发布日期 ⇒ 年份窗口不适用")
        self.assertEqual(spec.channel, "hbcpre")
        self.assertEqual(spec.origins, ("https://www.hbggzyfwpt.cn",))

    def test_both_list_entries_are_default_discovery_points(self):
        """挂牌与成交两个列表都是默认发现入口（一条命令两种列表都抓）。"""
        self.assertIn(JYGG_LIST, hbcpre.SPEC.default_list_urls)
        self.assertIn(JYJG_LIST, hbcpre.SPEC.default_list_urls)


if __name__ == "__main__":
    unittest.main()
