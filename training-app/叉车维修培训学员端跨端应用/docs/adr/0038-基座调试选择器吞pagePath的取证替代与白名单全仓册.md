# 0038 - 基座调试选择器吞 pagePath 的取证替代与白名单全仓册

日期：2026-10-13 · 关联：#1607（批4 N01 页头 19 页 + check-in 去 mock）· 承接：ADR-0035（页头件收口）、ADR-0016（①a 一次分支收口）

## 状态

已采纳（随 #1678 落地）

## 背景

①a 逐页真机取证需要把应用导航到指定页面。既有可靠通道（#937/#898 教训）只有 pages.json 首项临时换页；本批实测发现两条「看起来能用」的直达通道在此基座上是死路，且 1667 首轮 9 页 CLI 直达成功属偶发走通，不能当机制依赖。另发现三份页头契约套件各自内联页清单，白名单加条目后批 1 套件先红。

## 决策

1. **取证直达通道以「pages.json 首项临时换页 + plain launch」为唯一可靠机制。** cli launch `--pagePath/--pageQuery` 与 pages.json `condition` 块在基座 5.26.15075 均被内置调试选择页（UniActionSheetPage）拦截：该页 setup 阶段 NPE（`UniActionSheetPage.setup$lambda$18$lambda$7`，pages.kt:174）把交付吃掉，只剩空 ActionSheet 壳（点取消退 Launcher；连锁 `No package ID 96/3b/c4 found for resource ID`）。对照组（无 pagePath 的 plain launch）正常进首页——通道活、pagePath 交付死。此前某轮直达成功是偶发，不写进任何文档当机制。
2. **换页三纪律**：换前 ConvertFrom-Json 校验 + git diff numstat 应为 4/4（entry 对调）；数组末项无尾逗号坑（featured-detail 案）用 TrimEnd(',') 的无逗号版本对换；用毕从备份还原并 git status 确认 pages.json 干净。
3. **白名单是全仓册，防死条目并集必须跨文件同步。** utils/uiHexWhitelist.js 由多份契约套件共享，但各套件的「白名单值在受管页真实出现」自检只扫自己内联的 PAGES 并集——新批往白名单加值会让其他批套件误报死条目。新批 PAGES 落地时，必须同步补进**每一份**带该自检的套件（#1607 案：批 1 recruiterUiContract.test.js:135-154 补 BATCH4_PAGES 19 页后转绿）。
4. **转换类脚本的后验必须断言「结构在位」而非「字符串出现」**：批 4 换头脚本只验了 `<app-nav-bar` 标签存在，漏验组件 import 行，19 页全缺 import、③门 20 红。后验清单含 import 在位 + 机检先行（本地 jest 对抗自查后再交门）。

## 后果

- ①a 取证脚手架固定为换页机制，省去每轮通道试错；基座升级若修复选择器页，可重测直达通道后再改此 ADR。
- 白名单消费方清单在 utils/uiHexWhitelist.js 文件头维护，新增契约套件时同步登记。
- 契约套件内联页清单仍是重复真源（三份），收敛属后续重构票，不阻塞批 4。

**补注（2026-10-24，#1847 直达取证实践）**：换页机制的**结构边界**——`app.json` 首项换页带不了 query 参数，依赖入口 URL 参数的页（如 practice-do 的 `mode/count/tag_id/question_id`）**直达内容形态结构不可得**（冷启动停在参数缺失的空/等待态；路由级判据照过）。处置：如实记 skip + 接线守护兜底（#1697 login/register 先例同族），别反复重跑指望换运气；要内容取证须走带参入口的真链路（真机门/发版冒烟）。
