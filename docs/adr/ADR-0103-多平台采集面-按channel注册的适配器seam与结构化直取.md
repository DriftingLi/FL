# ADR-0103：多平台采集面——按 channel 注册的适配器 seam，与「结构化直取跳过 LLM」

- **状态**：已接受
- **日期**：2026-11-20
- **领域**：拍卖数据管线 / 采集层
- **票**：#2679（父 spec #2671）

## 背景

#2672–#2678 把整条管线建成了，但它只认一个采集面：`collector/cli.py` 的编排层里硬编码了 `parse_announcement_list`（列表）与 `parse_announcement`（详情），`models.CHANNEL_RMFYSSZC` 是唯一 channel 常量，`_build_crawler` 只读一个 base_url 的 robots。

#2679 要加京东与公拍网两个采集面，#2680 要加产权交易所省镜像。三者的形态差异极大：

| | rmfysszc | 京东 | 公拍网 | 产权交易所 |
|---|---|---|---|---|
| 列表 | 服务端表格 | 公告页隐藏 input | 服务端列表 / JSON API | 服务端表格 |
| 详情 | 公告正文 HTML | **JSONP 接口** | 服务端 HTML | 服务端表格 |
| 字段 | 非结构化正文 | 结构化 | 结构化 | 结构化 |
| robots | `Disallow: /` | **三处 host 全读不到** | 部分路径禁止 | **404** |
| 发布日期 | 有 | **无** | **无** | 有（公告期） |

所以本票要回答两个问题：**①多个采集面怎么接进同一条管线而不各写一份？②字段已经是结构化的采集面，还要不要过 LLM 抽取？**

## 决策

**一、采集面差异收敛成 `SourceSpec` 两个纯函数，编排层不认识任何平台。** `collector/sources/base.py` 定义：`parse_listing(html, list_url) -> [DiscoveredAnnouncement]` 与 `fetch_detail(url, fetch) -> [RawSample]`。除这两个函数外，`SourceSpec` 只剩识别与合规信息（`channel` / `origins` / `url_markers` / `structured` / `keep_undated`）。编排层（`cli.run_backfill` / `_process_url` / `_process_samples`）对所有平台一视同仁——**加一个采集面 = 加一个模块文件**。

**二、注册靠模块自注册 + 包内自动发现，因此两张并行票不抢同一行。** `collector/sources/__init__.py` 用 `pkgutil.iter_modules` 扫包内模块并 import，模块内调 `register()`。`#2680` 只需新增 `collector/sources/hbcpre.py`，不改 `__init__.py`、不改注册表、不改 CLI。这是 #2680 判据④「与京东/gpai 适配器无共享状态冲突（并行开发可各自绿）」的结构性保证。适配器模块只许 `from .base import ...`，不许 import 本包（半初始化陷阱）。

**三、`structured=True` 的采集面跳过 LLM 抽取，但不是跳过管线。** 京东的字段来自 JSONP 响应、公拍网与产权交易所的字段来自服务端渲染表格——它们**是来源事实，不是模型猜测**。硬要过一遍 LLM 只会引入无谓的失真与成本（且价格字段被模型改写是不可接受的）。所以 `_process_samples` 里只有一个分流点：

```
if spec.structured:  apply_structured(sample)      # 跳过 LLM，只判定 + 盖版本
else:                apply_extraction(sample, extract(...))
```

**分流点只有这一个**：清洗与匹配对两类采集面完全同一份代码。否则「结构化源不走清洗」这类漂移迟早发生。

**四、`apply_structured` 用与 LLM 路径同一套 `decide()` 判标签资格。** 价格门槛不能因采集面而放宽——没有价格就是没有标签。差别只在**抽检口径**：LLM 抽取面对整篇正文，某关键字段没抽出来属于抽取失败；结构化直取面对字段固定的数据源，字段缺席是**源站不提供**（公开渠道普遍没有「使用小时」，实测 12 个样本 0 个出现）。若照 LLM 口径把缺席一律算低置信，结构化样本会 100% 涌入抽检队列，把真正需要人看的价格问题淹掉。所以 `claimed_low_confidence_reason` 只审「适配器声称取到了」的字段。

**五、结构化样本盖 `extraction_version = "structured-v1"`，与 LLM 的 `PROMPT_VERSION` 分开。** 「这个字段是哪条路径产出的」必须在库里可分辨——重跑/审计时不能把来源事实与模型猜测混为一谈。

**六、robots 策略从「单 host」升级为「按 origin 各读一份」的 `RobotsBook`。** 这是多 host 采集面的**合规硬约束**，不是便利性改进：`RobotsPolicy` 按 host 绑定、遇到别的 host 一律 fail-closed（ADR-0101 决策八）。京东的公告页在 `paimai.jd.com`、JSONP 在 `api.m.jd.com`——拿前者的 robots 去判后者会把整个采集面判死。`RobotsBook` 每个 origin 各读一份、各自 fail-closed，按 URL 的 host 精确分派；**未核验的 host 仍 fail-closed**（「没读过」不等于「没有限制」），人工放行时记 `overridden` 审计。

**七、`robots.txt` 的「软 200」必须识别为「读不到」。** 实测三个平台都有这个陷阱：`paimai.jd.com/robots.txt` → 302 到 `www.jd.com/error.html`；`api.m.jd.com/robots.txt` → 返回 `{"code":"1",...}`；`zc.gpai.net/robots.txt` → 返回 SPA 首页 HTML。这些文本里没有任何 `Disallow` 行，直接交给 `parse_robots` 会解析成「无规则 ⇒ 全部允许」——把「站点没给我们 robots」误判成「站点允许一切」，是合规上的反向错误。所以 `_looks_like_robots` 先判形态（HTML / JSON / 空 / 无任何 robots 指令 ⇒ 视为读不到 ⇒ fail-closed）。

**八、`keep_undated` 让结构化采集面绕过年份窗口。** 回补（rmfysszc）靠发布日期卡 2015–2026 窗口，无日期无法归年必须丢（ADR-0101 决策七）。但京东/公拍网的标的**根本没有发布日期字段**（京东接口只给 startTime/endTime 的 ms 时间戳，公告页不给）——若照同一条规则过滤，它们的样本会被 100% 丢光，表现为「回补成功但 0 条」：没有报错、没有 failed_urls、得率报告显示「本轮无样本」。这是最难查的那类静默失败，所以 `SourceSpec.keep_undated` 显式声明，且有专门的回归测试（`UndatedSampleSurvivalTest`）。

**九、适配器必须把「反爬拦截」识别成抓取失败，不能解析成空样本。** 实测 2026-11-20 14:39 起 gpai 对本 UA 返回 1,774 B 的滑块验证码页（`seccaptcha.haplat.net`），页面 `infoString` base64 解出 `{"type":"blacklist", …}`——诚实 UA 已被拉黑。适配器若返回 `[]`，得率报告会全绿而库里一行没有。所以 `parse_sf_detail` 命中验证码指纹即抛 `CaptchaBlocked`，由编排层的毒丸隔离接住并记进 `failed_urls`。

**十、适配器必须填清洗规则要用的判据字段，否则规则永不命中。** 这一条是**真实抓取暴露出来的缺陷**，不是设计时的预见：京东破产/资产处置通道的标题实测几乎全是「报废资产…【无电池】」形态，而清洗规则②（`_rule_scrapped`，废铁价 ≠ 使用价值）的判据是 `structured_specs.scrapped` / `has_battery is False`。适配器不填这两个字段，规则②就没有判据可看，**废铁价会被当成市场成交价落成 cleaned 样本**，直接污染下游残值率校准。所以适配器按标题线索填字段（且只在**明确命中**时填，不命中留空——「没提到报废」≠「车况良好」）。教训：结构化采集面的「字段映射」不只是抽规格，还包括清洗规则赖以工作的判据字段。

## 平台能力边界（实测，写进 ADR 以免后人反复试错）

**京东**：可用 JSONP 接口**只有两个**——`getProductBasicInfo` 与 `getPaimaiRealTimeData`。其余（`getProductBidInfo` / `getBidList` / `getProductDealInfo` / `getPaimaiResult` / `getProductInfo` / `getAuctionResult`）一律返回 `{"code":"2","echo":"the current API does not exist"}`，**不要猜接口名**。标的页 HTML 是 SPA 空壳（78,393 B，无任何价格 key）⇒ 必须走 JSONP。

**京东判据④（破产通道）的形态与原票假设不同，按实测修正**：原票假设「破产通道不公示确认书」可作判据，但实测 7 个样本（含已结束场次）**全部** `auctionStatus=2`、`confirmationUrl=""`——`auctionStatus=2` 是常态值、`confirmationUrl` 空串也是常态，两者都不能用来判「已成交」或「破产通道」。真实可用的判据是**评估价是否为零**：破产/资产处置通道（auctionType=5、businessType=1、platType=4、publishSource=9）的 `assessmentPrice` 实测全为 0.0（7/7），清算价字段全为空串。所以判据④落地为：**评估价为 0 时归一成 `None`（「源站不提供」而非「零元」），照常落成交价，只跳过溢价比较**。这不是妥协——清洗规则⑦本来就只在 `appraisal` 为真值时才比较。

**公拍网**：司法频道 `/sf/` 与 `/sf/item2.do` 可采（robots 未禁），但**已成交样本只在 `s.gpai.net/sf/search.do` 上**，而该路径被 `www.gpai.net/robots.txt` 明示 `Disallow`，且 `s.gpai.net/robots.txt` 拿不到（302 返回京东首页 HTML，异常镜像）⇒ **已成交样本在合规默认下不可采，必须 `--allow-robots-override`**。资产频道 `zc.gpai.net` 是 SPA，列表接口 `/api/item/list` 可用但 **pageSize / pageNum / keyword 参数全被忽略**（只能取到一页），详情接口全废（`info为空` / `data:null` / `未登录`）⇒ **资产频道只能落「在拍标的 + 起拍价」**（`listing`），拿不到成交价与评估价。

**判据③的验证偏差（必须记明）**：实测扫描 gpai 司法频道 60 个已结束标的，`成交价/评估价 > 1.5` 命中 **0 个**——司法拍卖成交价普遍是评估价的 0.65~0.72（4 个实测样本：0.653 / 0.716 / 0.713 / 0.667）。所以「溢价 >评估价×1.5 标记在真实形态上生效」**无法靠真实语料验证，只能靠 fixture 构造**（`PremiumRuleOnRealShapesTest`）。这是平台语料的事实偏差，不是实现缺陷。

**京东的 robots 三处 host 全部读不到有效 robots.txt**（soft-200 陷阱，见决策七）⇒ 按 ADR-0101 决策二 fail-closed，采集必须显式 `--allow-robots-override`，且每次请求记 `robots_override` 审计。

## 被否备选

- **每个平台写一条自己的管线**：最直接，但正是 spec #2671 要禁止的——清洗/匹配必然漂移，同一标的不同来源会得出不同残值率标签。
- **结构化采集面也过 LLM 抽取**：字段本来是 JSONP/表格里现成的，过一遍模型只会引入失真与成本；价格字段被模型改写尤其不可接受。
- **`SourceSpec` 里放「抓取函数」而不是 `fetch_detail(url, fetch)`**：那样适配器可以自己 import urllib 绕过 `CrawlClient`，robots/限速/退避会被悄悄跳过。现在适配器只能拿注入的 `fetch`，**合规三道闸无法绕过**（有测试钉住：`test_all_requests_go_through_injected_crawler`）。
- **`_build_crawler` 读所有已知 origin 的 robots**：会在无关站点上多发请求。按「本轮真正要抓的采集面」收集 origins 最准。
- **把无日期条目当成「窗口外的旧公告」丢弃**：会让结构化采集面静默落 0 行（见决策八）。
- **验证码页返回空样本、让上层当「本页无标的」**：得率报告全绿而库里一行没有（见决策九）。
- **在编排层按 channel 写 if/elif 分派**：每加一个采集面都要改编排层，正是 seam 要消除的耦合。

## 后果

**收益**：①加采集面 = 加一个模块文件，编排层/CLI/注册表零改动（#2680 可直接复用）；②清洗与匹配对全部采集面同一份实现（只有一个分流点）；③合规三道闸无法被适配器绕过；④多 host 采集面的 robots 粒度正确（每 host 各自 fail-closed）；⑤结构化样本的标签资格门槛与 LLM 路径一致（价格是硬判据）；⑥来源路径可审计（`extraction_version` 区分）。

**代价**：①`SourceSpec` 的 `parse_listing` 必须收 `list_url`（相对链接要拼绝对地址，而绝对地址是幂等键）——签名比「只收 HTML」复杂，但这是必要的；②结构化适配器要**手工维护字段映射**，平台改字段名/改标记时没有编译期保护，只能靠录制 fixture 的解析测试先红；③`keep_undated` 是按采集面整体声明的粗粒度开关——若某个结构化平台将来有发布日期，需要改成按条目判断；④公拍网已成交样本在合规默认下不可采、资产频道只有列表字段、京东零评估价——这三条都是**平台能力边界**，意味着 #2679 对 gpai 的贡献主要是「在拍标的 + 起拍价/评估价」，成交价一侧主要靠京东与（后续的）产权交易所；⑤实测样本里 gpai 司法频道 0 个溢价样本，判据③只有 fixture 覆盖，真实语料的溢价行为仍未知。

**实施边界（本票不做）**：产权交易所省镜像（#2680）、阿里拍卖（spec 明确后置二期）、已结束列表的 cookie jar 维护、验证码绕过（不做，也不应做）、管理端 UI、评估路径改动。

**后续票线索**：①公拍网已成交列表需要 cookie jar（先访问 `www.gpai.net/sf/` 拿 cookie 再打 `s.gpai.net`），当前适配器未实现——因为该路径本身在合规默认下被禁；②`zc.gpai.net` 详情接口参数名未能探明（SPA bundle 内无 `itemId` 字面量），若将来平台开放可补；③京东公告页的发现入口依赖专场公告 URL，尚无「叉车类专场」的自动发现机制（需人工给 `--list-url`）。
