# #1677 取证会话 run16 关键行（cert-fix-evidence.mjs）

- 日期：2026-10-14（小程序优先窗口，ADR-0039）
- 分支/提交：feat/1677（源码 14:25 定稿 → 16:22 publish 产出 mp-weixin 产物；同内容树入库为 398e9b5f，首提 c4a481b5 经 amend 并入，差异仅本文件）
- 会话：`.ci-verify/evidence-session-1677.ps1` → `scripts/cert-fix-evidence.mjs`（ws://127.0.0.1:9420）
- 配套帧：`dashboard-overlay-open.png`（开态，83472B）· `dashboard-overlay-closed.png`（闭态，76197B）

## 关键行（原样摘自 evidence-session-run16.log）

```text
ev| MOCK dispatch grouped+me/credential=production其余=空信封(旧行为)
ev| CERT_KEYS if=e uP=h
ev| NAV_BIND titleBlockBindtap=true path=unpackage/dist/build/mp-weixin/components/app-nav-bar/app-nav-bar.wxml
ev| TOGGLE_CANDS ["e0_cf","e1_30","e2_9e","e3_a7","e4_25","e5_39","e6_2f","e7_47","e4_2d","e5_25","e6_39","e7_2f","e8_47"]
ev| TOGGLE_TRY e0_cf res=called opened=true route=pages/dashboard/dashboard
ev| TOGGLE_VIA e0_cf
ev| OPEN via=app-toggle（h=e.p 引用，无 e1.split）
ev| CERT_TRUTH ok=true via=app-toggle(uP=ref-string) overlay=true —— chips 视觉见开态帧 + console grouped 200
ev| EVIDENCE closed=76197B open=83472B
ev| CONSOLE count=33
ev| EXCEPTIONS count=0
ev| CORE_EVIDENCE captured（核心证据已落袋；此后元素实验挂死也不影响①②的判定）
ev| TAP_TARGET via=xpath resolved=true
ev| TAP_RESULT via=xpath opened=false（tap 送达后 wx:if 仍 false —— 见 CONSOLE toast）
EVIDENCE_SESSION_OK
```

## 对验收条的映射

- **票面 ②（EXCEPTIONS=0 + 双帧）**：`EXCEPTIONS count=0` + 上述双帧入库 `docs/verification/mp-weixin/1692/`。
- **票面 ①a 第一桶（浮层见真实 chip，两桶各 ≥1、currentId 高亮）**：开态帧内可直接目视——「特种作业配套上岗证」3 chip（叉车司机N1证蓝底高亮）+「工程机械维修工（叉车维修方向技能等级证）」5 chip（五级…一级）；数据链由 `console grouped 200` 与 `TOGGLE_VIA e0_cf` 置真后的 `h=e.p` 引用共同背书。
- **票面 B 链（页头入口→开浮层）**：`NAV_BIND titleBlockBindtap=true`（B 进编译产物）+ `TOGGLE_TRY e0_cf res=called opened=true`（页面真 handler 即 @left-click 绑定的 onCertTap，调用即开浮层）。
- **已知缺口（诚实记录）**：automator 物理 tap 信道在本机恒不触发（`TAP_RESULT opened=false`，#883 元素信道同族；CONSOLE 无「证件数据加载中」toast 说明 tap 未走到 handler）⇒ 真机点按开关的运行时实证按 ADR-0039 窗口口径落到 ①a 批收口（PR ① 行 `SKIP·待批收口`）。
- **e1.split 结论**：仅由旧版脚本「裸喂 h=对象」触发（`Fi(对象).split` 抛错，取证伪象）；产品路径 `h` 是 `e.p()` 引用字符串、拆串正常——run16 全程 `EXCEPTIONS count=0` 实证。
