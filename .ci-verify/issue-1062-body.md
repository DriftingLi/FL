## 进度：100%

已完成并合并：**PR #1069**（squash 合入 master `e29e203a`，2026-09-16）。

验收结果：③ `npm run test:unit` 全绿（79 suites / 1284 tests）· ④c `npm run build:kotlin-all` `KOTLIN_ALL_RESULT errors=0 classes=1300 files=98` · ①a 真机取证完成（设备 `f0bae674`，改动页 `--pagePath` 导航落定 + 本地 OCR 逐字判据）—— 纯文案改动、未命中能力面 ⇒ **①b 不触发、无人签**（`CAPABILITY_SURFACE touched=no`）。PR 上 `pr-evidence`、`mobile-test`、`ci-summary` 均绿。

## 现状（2026-09-16 校正）

票面原「现状（三处文案）」表是**对着未合并的 PR #1046 分支**写的：行号与「承重约束」在 master 上都不成立。校正后的真源（`origin/master` @ 30b6e163，本次开工基）：

| 位置 | 改前文案 | 处置 |
| --- | --- | --- |
| `pages/ai-assistant/ai-assistant.uvue:262` | `uni.showToast`「暂无可用的 AI 模型」 | **改**（本次交付物） |
| `pages/ai-assistant/ai-assistant.uvue:266` | `uni.showToast`「请先配置自定义模型」 | 不变（已指向具体动作） |
| `pages/ai-assistant/ai-settings.uvue:56` | 空态「暂无平台模型，请联系管理员配置」 | 不变（已指向可执行动作：联系管理员） |

## 承重约束（2026-09-16 校正：票面原判断不成立）

票面原文说「这三处文案被 3 个契约测试锁着，只改页面不改测试 ⇒ `npm run test:unit` 直接判红」。**实测不成立**：

1. `utils/aiDiagnosisSourcesContract.test.js` —— **在 master 上不存在**（它是未合并 PR #1046 的新增文件），票面引的 `GUARD_RE`（`...:290`）随之不存在。
2. `utils/aiFeatureEntryWiringContract.test.js` —— 票面说 `:138-139`「断言这两条文案字符串存在」。实测该行断言的是**类型表**（`featureKey : string` / `freePreview : boolean`）；全文对这两句只有一处**负向**断言（`:334` 对专项页 `not.toMatch(/暂无可用的 AI 模型/)`）与一处**条件**断言（`:234` 钉的是第二道守卫的**条件式**，不是文案）。
3. `utils/aiFunctionEntryContract.test.js:146-149` —— 断言的是**专项页不得**出现这两句（负向），与通用页那两句无锁关系。

⇒ 改这两处文案**不会**让任何既有测试变红；票面「同步 3 处契约测试」这一交付物**无从同步**。

## 口径（2026-09-16 维护者裁定，票面「默认值」经实测改定）

票面给的默认句「请在「设置」中配置自定义模型或联系管理员」经实测有两处不成立，故按维护者裁定改为：

**`平台模型未配置，请联系管理员；专业版可在「对话设置」中配置自定义模型`**（34 字）

1. **入口名不成立**：票面写「设置」，而真实入口名是 **「对话设置」**（`ai-assistant-constants.uts` 的 `RIGHT_MENU_ITEMS` 里 `settings` 项的 label，也是该页标题）。
2. **对未解锁学员是死路**：自定义模型的入口与配置面**都在专业版门内** —— 未解锁时右侧菜单把 `custom-models` 项过滤掉（`ai-assistant.uvue` 的 `rightMenuItems`）、设置页的 `user` / `custom` 来源整行不渲染（`ai-settings.uvue`，#1024）
   ⇒ 只写「去配置自定义模型」对未解锁学员**一步都走不通**，那正是本票要防的缺陷。故**先**给对**所有**学员都可执行的「联系管理员」。
3. 第 2 条（`请先配置自定义模型`）：**不变**。
4. 第 3 条（设置页空态）：**不变** —— 它已指向可执行动作（联系管理员）。

## ①a 真机实测发现（这条是本次唯一的「计划外」收获）

真机（Redmi 23049RAD8C / `b32d8398`）实测：**`uni.showToast` 的文本区是「两行 × 18 字」上限**。

- 首版按裁定交付的是 37 字（`…；已解锁专业版可在「对话设置」中配置自定义模型`），真机 OCR 三帧一致地显示**末字「型」不渲染**（每行恰好 18 字）。
- 处置：删「已解锁」压到 **34 字**（语义不变），复测 OCR 逐字完整 —— `平台模型未配置，请联系管理员；专业版` / `可在「对话设置」中配置自定义模型`。
- 已把这条例外写进代码注释，并**落成守护**（`aiEmptyStateActionableContract.test.js` ④「字数上限 ≤36」）。

## 明确不做

- **不动守卫的判定条件**：`models.value.length == 0 && userModels.value.length == 0 && currentModelSource.value != 'custom'` 保持原样（#998 / #1045 口径）。
- 不改 Web 端（那是 #1061，已由 PR #1067 合并）。
- 不改后端。
- **不引入按 `proUnlocked` 分支的两套文案** —— 那超出票面「只改文案」的范围（被否备选见下）。

## 验收

- ③ `npm run test:unit` 全绿（合并 master 后：79 suites / 1284 tests）。
- 改动含 `*.uvue` ⇒ 运行时面 ⇒ ④（`npm run build:kotlin-all`）。
- ① 按 `docs/adr/0016-真机门的人工性收缩与按批取证.md`：纯文案改动、未命中能力面 ⇒ **①b 不触发、无人签**；**①a 由 agent 出证**（入仓截图 + OCR 文本判据）。`dev:finish` 的能力面提示行实测 `CAPABILITY_SURFACE touched=no`。
- 新增 `utils/aiEmptyStateActionableContract.test.js`：锚点取守卫**条件**（不取文案，避免守护退化成文案快照 —— 票面「建议（推荐做）」那条的要求），钉「文案必须指向可执行动作」+ 字数上限 + 变异自检（反例：换回旧裸文案必须判红，已实测）。

## 交付物

- `pages/ai-assistant/ai-assistant.uvue`（改文案 + 记录字数上限的注释）
- `utils/aiEmptyStateActionableContract.test.js`（新增守护，5 例）
- `docs/verification/ai-assistant/1062/`（①a 入仓截图：toast 在屏 / toast 消失两帧）

## 关联

- 来源：#1044（❓Q2 裁定「要硬化」；终态「通用对话接受不可用」）
- 姊妹票：#1061（Web 空态；已由 PR #1067 合并 —— Web 侧的「设置」入口与移动端不同，其文案未按移动端的专业版门改写）
- 已作废的前提：#1045（客户端命题已随 PR #1059 结构性消解）、PR #1046（其 `featureKey` 豁免在 #1059 后已不适用，master 上从未落地）
