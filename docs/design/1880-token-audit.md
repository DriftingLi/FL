# #1880 token 审计：固定色阶当填充，深色下读不清

> 起因：修欢迎条（#1880 / #1880 的前身 #1750）时发现一个模式 —— `--color-primary-50/100` 这类**固定色阶**在
> `[data-theme="dark"]` 段里没有重定义，一旦拿它当**填充**，深色下就是一块浅色底。本次把全站扫了一遍。

## 判据（两条，缺一不可）

1. **token 翻不翻转**：`design-tokens.css` 的 `[data-theme="dark"]` 段有没有重定义它。
   - 翻转（安全）：`bg-page` / `bg-card` / `border*` / `text-*` / 语义 `*-light` 与 `*-strong`
   - **不翻转**（危险）：`primary-50..900`（= 原子类的 `ui-*`）、`accent-*`、`violet-*`
2. **实际对比度**：填充色与它上面**真正生效的墨色**的 WCAG 对比度。深色下正文 ≥ **4.5**，tertiary / 占位 ≥ **3.0**。

只判第 1 条会把「填充固定 + 墨色也固定」的成对 chip 也算成 bug —— 那类**可读性不变**，只是"深色下亮一点"。
所以按「填充是否固定 × 墨色是否翻转」二分，才有意义。

## 扫描口径与结果

扫 `frontend/src/**/*.{vue,ts,css}`（排除 `__tests__`）：
- 原子类：`(bg|from|via|to)-(ui|ui-accent)-<50..300>`（含 `hover:` 等变体前缀）
- CSS：`var(--color-primary-*|accent-*|violet-*)` 且该行是 `background / gradient / fill / box-shadow`

| 分类 | 判据 | 数量 | 处置 |
| --- | --- | --- | --- |
| **A 类** | 填充**固定** + 墨色**翻转** | **13 处 / 9 文件** | **本次修** |
| **B 类** | 填充固定 + 墨色**也固定**（成对） | **31 处 / 18 文件** | 另票（要新 token 配对） |
| C 类 | `text-*` / `border-*` / 深色阶（≥400） | 227 处 | 不在射程 |

## A 类 13 处：深色下对比度 1.05 ~ 2.46（等于看不见）

实测（脚本渲染「修前 / 修后」同一墨色两版，浅/深各取一次，`contrast.json` 是原始数据）：

| 位置 | 修前填充 | 墨色 | 浅色 前→后 | **深色 前→后** |
| --- | --- | --- | --- | --- |
| `recruit/OnlineResumePdf.vue:4`（420px 骨架） | `bg-ui-50` | `text-ink-3` | 4.56 → 4.13 | **2.46 → 4.94** |
| `student/ResumePage.vue:11`（浏览提示条） | `bg-ui-50` | `text-ink` | 17.12 → 15.49 | **1.05 → 11.56** |
| `student/CheckInPage.vue:99`（我的排名条） | `bg-ui-50` | `text-ink` | 17.12 → 15.49 | **1.05 → 11.56** |
| `student/CheckInPage.vue:251`（今日格） | `bg-ui-50` | `text-ink` | 17.12 → 15.49 | **1.05 → 11.56** |
| `student/CheckInPage.vue:252`（已打卡格） | `bg-ui-100` | `text-ink-2` | 6.72 → 6.11 | **1.32 → 7.88** |
| `tutor/QuestionTags.vue:356`（`.tag-row.active`） | `primary-50` | `text-primary` | 17.12 → 15.49 | **1.05 → 11.56** |
| `student/QuestionOptionPicker.vue:51`（`.q-option.selected`） | `primary-50` | 继承 `text-ink` | 17.12 → 15.49 | **1.05 → 11.56** |
| `tutor/TutorChapterEdit.vue:100`（选中文件行） | `bg-ui-50` | `text-ink` | 17.12 → 15.49 | **1.05 → 11.56** |
| `onboarding/CredentialOnboarding.vue:183`（证件卡**基底**） | `white` | `text-ink` | 17.85 → 17.85 | **1.10 → 13.35** |
| `student/AnsweringSessionShell.vue:145`（答题工具条） | `#fff` | `text-ink` | 17.85 → 17.85 | **1.10 → 13.35** |
| `student/VideoPlayer.vue:272`（`.video-error`） | `#fff` | `text-ink` | 17.85 → 17.85 | **1.10 → 13.35** |

> 表里 11 行对应 13 处改动：`CredentialOnboarding` 还有 `:hover` / `.active` 两处填充（同一文件），
> 与基底一起改。

**读法**：深色下修前是 1.05 —— 那是"白字压在近白色的底上"，不是"有点亮"，是**根本看不见**。
`CredentialOnboarding` 尤其严重：它是**卡片基底**（`background: white` 裸值），深色下整页证件卡都是白底白字。

修后深色 4.94 ~ 13.35，全部过 AA；浅色侧除骨架 4.56 → 4.13（占位文字阈值 3.0，仍达标）外基本不动。

## 修法

- 填充：`var(--color-primary-50)` → `color-mix(in srgb, var(--color-primary-500) 12%, var(--color-bg-card))`
  （`primary-100` 档用 **18%**）；原子类写 `bg-[color-mix(in_srgb,var(--color-ui-500)_12%,var(--color-panel))]`。
- `background: white` / `#fff` → `var(--color-bg-card)`（表面 token，本来就会翻）。
- **墨色一律不动** —— A 类的墨色本来就是翻转 token，问题只出在填充。

## B 类 31 处（另票，本次不改）

填充固定 + 墨色也固定的**成对**写法，例如 `bg-ui-50 text-ui-600`、`bg-ui-100 text-ui-700`：
深色下它们是一块浅青底压着深青字，**对比度不变、读得清**，但视觉上仍是"暗页面上的亮块"（#1750 说的刺眼白块，
小尺寸版）。分布：AI 助手会话壳 5、签到页 2、收藏/图床/课程/题库/帮助 7、UiDialog / UiStatCard / UiTag 3、
侧栏与顶栏 3、凭据切换 2、`FacetItem` / `QuickCard` / `FeatureChatPage` / `Materials` / `global.css` 选区 /
`element-overrides.css` 下拉 hover 各 1~3。

**不建议逐个改**：一旦把填充换成翻转的深底，这些地方固定墨色（`text-ui-600`）就会变成"深底深字"。
要收就得像语义色那样成对引入会翻转的 token：

```css
--color-ui-soft: color-mix(in srgb, var(--color-primary-500) 12%, var(--color-bg-card));  /* 填充 */
--color-ui-soft-ink: var(--color-primary-700);   /* 浅色下；深色段重定义为 primary-300 之类的浅青 */
```

照 `ok-soft` / `ok-strong` 的既有做法（那两个本来就翻），然后 `bg-ui-50 text-ui-600` → `bg-ui-soft text-ui-soft-ink`。
那是**设计系统改动**（新增一对 token + 迁 31 处），需要设计侧拍板，单独立票。

## 复现

```powershell
cd frontend
node ../.scratch/audit-tokens.mjs        # 固定色阶 × 面积填充 全量清单
node ../.scratch/audit-classes.mjs       # 原子类那一路（含 hover: 等变体）
# 对比度：临时脚手架 harness-1880b（修前/修后并排）+ node verify-1880b.mjs
#         取 computedStyle 的 backgroundColor / color 算 WCAG 比值
```

截图：`docs/verification/tooling/1880/tokens-light.png` / `tokens-dark.png`（11 组修前/修后并排），
原始比值 `docs/verification/tooling/1880/contrast.json`。
