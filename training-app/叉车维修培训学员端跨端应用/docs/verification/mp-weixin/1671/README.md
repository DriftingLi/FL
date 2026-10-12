# #1671 ①a 证据 — cert-selector 浮层 mp-weixin 绘制修复（fixed → absolute）

> 证据面：微信开发者工具 automator（真机不可用，先例 #1473 口径：devtools 模拟器截图 + console 零报错；像素判据归 Android ①a——本修复是纯样式层修复，Android 侧 position:absolute 先例 .filter-mask 已在真机长期验证）。
> 修复：components/cert-selector/cert-selector.uvue 的 `.cert-dropdown-mask` `position:fixed → position:absolute`（锚定宿主 .container；inset 四向 0 / z-index 300 / rgba(0,0,0,0.4) / 点击关闭语义全保留）。
> 复现口径：connect ws://127.0.0.1:9420 → mockWxMethod('request') → switchTab('/pages/dashboard/dashboard') → 渲染层 setData 直写 v-if 键 e（automator 无法穿透 custom component 边界触发 onCertTap；e = dashboard.wxml 中 `<cert-selector wx:if="{{e}}">` 的渲染键，与真实入口 AppNavBar 左槽 tap 共享同一 v-if 与组件树）。

## 机检行（png-lum-profile.mjs，与 #1650 同判读口径）

| 帧 | 文件 | median | darkFrac | 白面板行范围 | mask |
|---|---|---|---|---|---|
| 修复前基线（#1650 12/12 帧） | docs/verification/mp-weixin/1650/ | — | ≤0.03 | 7–8% 屏高 | NONE（遮罩一帧未画） |
| 修复后·关闭态（未开浮层） | dashboard-overlay-closed.png | 227 | 0.00 | [665..898] | NONE（无浮层） |
| 修复后·开启态（浮层已开） | dashboard-overlay-open.png | 246 | 0.00 | [499..898] | 面板 [499..898] = **40% 屏高满屏面板** + 顶部暗条（40% 黑遮罩压暗状态栏） |
| 判读器判别力对照（dashboard 自身 filter 抽屉，absolute 先例，遮挡后变暗判读正确） | 探针会话帧（已删，判读值见 git 历史探针 8 记录） | 137 | — | [680..723] | SHALLOW（判读器正确识别暗层） |

机检结论：**修复后浮层开启时面板绘制 40% 屏高（vs 修复前 7–8%）、状态栏区被 40% 黑遮罩压暗（band0 146 vs closed 131）、console 零异常（EXCEPTIONS=0）、request mock 下零 401 弹人。修复前三形态 mask=NONE（#1650 12/12 帧）→ 修复后开启态顶部出现遮罩暗条 + 面板满屏。**

## OCR 交叉验证（modlens）

- closed 顶部：状态栏 + 「选择证件 ▼」+ 铃铛 + 「叉车维修培训开课通知」banner = dashboard 原生内容，无浮层。
- open 顶部：**深色状态栏条 + 「选择学习阶段」标题 + ✕ + 热门考证双蓝卡**（入门组合/电动叉车维修优选）= cert-selector 面板满屏渲染。
- open 底部：白面板直达 tab bar 上缘，五标签（首页/课程/AI学/交流/我的）可见 = 与原生 tabBar 平台约束一致（#1650 判定：full 满屏结构上不可达 tab bar 之上）。
- closed 底部：无抽屉、无遮罩，纯 dashboard 内容。

## 工件清单

- dashboard-overlay-closed.png / dashboard-overlay-open.png（461×997 整页）
- crop-closed-top.png / crop-open-top.png（0-260）、crop-closed-bottom.png / crop-open-bottom.png（697-996）
- 判读：scripts/png-lum-profile.mjs（自 wt-1650 复制，与 #1650 同口径）、scripts/png-crop.mjs
- 取证脚本：scripts/cert-fix-evidence.mjs（v4：sweep 复位 + closed→open 双帧 + console/exception 采集）
- console 采集：CONSOLE 3 条（request mock 200×2 + getSystemInfo deprecation warn），EXCEPTIONS=0

## 与 #1650 判读结论对照

- #1650：12/12 帧 mask=NONE、面板 7–8% 屏高、tab bar 全程可见 → 结论「面板渲染不出、遮罩不绘制」。
- #1671 修复后：面板 40% 屏高、顶部暗条实证遮罩绘制、tab bar 可见（平台约束内）→ **同判读口径下三项指标全部反转**。
