# #937 交付说明与门证据（uvue 渐变语法收敛）

> 归档时间：2026-09-14 ｜ 分支：`fix/937-gradient-syntax` ｜ PR：https://github.com/DriftingLi/FL/pull/972
> 关联：issue [#937](https://github.com/DriftingLi/FL/issues/937) ｜ ADR：`docs/adr/0010-uvue渐变语法约束.md`

## 一、做了什么

全项目 **22 处** `.uvue` 渐变声明（18 个文件）中，**15 处**用了 uvue 原生端不接受的写法（带百分比停靠位 / ≥3 个颜色值），在真机上被**整条静默丢弃**（不报错、不警告），该有底色的地方直接露白。

本 PR 把它们统一收敛为**恰好 2 个颜色值 + 无百分比停靠位**，方向写法**保留原样**（角度已实测合法），并逐处补 `background-color` 兜底（写在 `background` 简写**之后**）。

## 二、真机实测依据（诊断阶段探针页，10 种写法）

| 写法 | 实测 | 判读 |
| --- | --- | --- |
| `linear-gradient(135deg, #e3f0ff, #cfe4ff)`（角度 / 2 值 / 无 %） | `#D3E3FC` | ✅ 可用 |
| `linear-gradient(to bottom, #FF0000, #0000FF)`（关键字 / 2 值 / 无 %） | 红→蓝渐变 | ✅ 可用 |
| `linear-gradient(135deg, #FF1493 0%, #FF1493 100%)`（带 %） | `#FFFFFF` | ❌ 整条丢弃 |
| `linear-gradient(180deg, #CFE9FB 1%, #D0EBFD 16%, #F5F5F5 100%)`（3 值 + %） | `#FFFFFF` | ❌ 整条丢弃 |
| `linear-gradient(to bottom, #F00, #FF0, #0F0, #00F)`（4 值 / 无 %） | `#FFFFFF` | ❌ 整条丢弃 |

⇒ 两条被证伪的旧结论（此前写在 3 处源码注释 + 2 个契约测试 + 1 份对齐成果文档里）：
**① 「uvue 不绘制 `linear-gradient`」**、**② 「角度（`deg`）不合规」**。均已在本 PR 订正。

**未单独实测的一点，不写成定论**：「兜底是否真能兜住」—— 探针里兜底写在简写**之前**（会被简写重置），与「整条被丢弃」不可分辨，故 ADR 0010 只规定书写顺序、不宣称兜底一定生效。

## 三、两道门的产物

### ③ `npm run test:unit`

`51 suites / 899 tests` 全绿，含新增守护 `utils/gradientSyntaxContract.test.js`（12 例：注入自检 5 + 合规样本 6 + 真实文件零命中 1）。

### ④c 本地编译门（整模块 Kotlin 编译）

`KOTLIN_ALL_RESULT errors=0 classes=1255 files=95`，证据为 sha 绑定评论（`<!-- gate-evidence:④ -->` + `commit: 0294fcc5`）：
https://github.com/DriftingLi/FL/pull/972#issuecomment-5654523235

**编译产物复核（本文件新增）**：从 `unpackage/**/*.kt`（190 个文件）提取全部 `backgroundImage` 渐变声明 —— **42 处，非「2 值」或含 `%` 的 = 0，且 42/42 都带实色兜底**。

复核逻辑（可复现，按顶层逗号剥掉方向后再数颜色值）：
```python
# 关键点：不能对整串用 [^)]* —— rgba() 内含逗号会截断；也绝不能把方向算成颜色值
parts = [x.strip() for x in args.split(",")]
stops = parts[1:] if DIRECTION.match(parts[0]) else parts
assert "%" not in args and len(stops) == 2
```

这条**不替代 ①**（渲染对不对只能真机看），只证明改动的值确实进了编译产物、未在编译期被归一化或丢掉。

## 四、① 真机门（待签收）

**状态：待人工签收**（agent 未代填执行人、未写「已通过」）。

待复看对象：
1. `pages/profile/profile` 的 `.container`（整页底色；修复前真机实测**纯白**，最显眼）；
2. 任一 tabBar 页（`courses` / `dashboard` / `forum`）页底 —— 由「3 色 + 百分比」改为 `#CFE9FB → #D0EBFD`。

## 五、判据与纪律备注

- **计数口径**：真声明 = 22 处 / 18 文件（「排除纯注释」+「剥方向再数颜色值」两条缺一不可；朴素正则会假阳性 22/22）。
- **兜底顺序**：CSS 简写 `background` 会重置未列出的 `background-color` ⇒ 兜底必须写在**之后**（本 PR 复查发现并修正了 22 处）。
- **`.uvue` 不支持 CSS 变量** ⇒ 渐变无法收成一个 token，只能逐处写字面值（这是「同一写法重复 22 处」无法消除的原因，已记入 ADR 0010）。
