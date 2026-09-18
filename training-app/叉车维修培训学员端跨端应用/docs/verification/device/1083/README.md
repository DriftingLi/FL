# #1083 真机取证（①a）—— **未通过**：到达了错题本页，但从未进入卡片列表

> 采集时间：2026-09-17 · 被验树：`feat/mobile-1083-wrong-question-card` HEAD `c2064303`
> 设备：`f0bae674`（23049RAD8C / Android 15）· 应用：`io.dcloud.uniappx`（apps/`__UNI__1C1D180`）
> 部署判据：`HX_RUN mode=incremental compile=150 deploy=0 total=211 exit=ok`，
> 设备侧 `www` mtime `1789630063 → 1789631318`（相对基线前进）

## 更正（同日；本文件首版有错，勿再引用首版）

首版把截图里的元素当成「错题卡片列表的计数带」，并据此写了「与卡列表形态一致」。**该推断是错的。**

定量判据（本机 rpx→px 系数 = 1156/750 = 1.5413）：

| 量 | 值 |
| --- | --- |
| 截图里蓝色圆形实测 | 宽高各 **74 px**（bbox `y=848-921, x=76-149`） |
| 分组视图 `.group-icon` | `48rpx` = **74.0 px**（`width/height/border-radius:24rpx`、`border:3rpx solid #2979ff`） |
| `.group-icon` 左偏移 | `group-list` padding `24rpx` + `group-card` padding-left `24rpx` = `48rpx` = **74.0 px**（实测左缘 76 px，含描边取整） |
| 出现次数 | **每行恰好 1 个，共 3 行** |

⇒ 屏幕上是**分组视图的 3 个分组行**（`.group-card` + `.group-icon`），不是卡片列表。

反向确认：卡片列表每张卡在**右半区**有操作行（实心 `#2979ff` 的「重做」+ `#f44336` 描边的「移出」，
本票又加了同样描边的「查看答案与解析」），而对 `wrong-questions.png` 右半区（x>620）扫
`#2979ff` 与 `#f44336` 两族像素，**命中数为 0** ⇒ 那一帧里根本没有卡片。

成因（源码事实）：`pages/profile/wrong-questions.uvue` 的 `const grouped = ref<boolean>(true)`
—— 该页**默认就是分组视图**，要 `@click="onGroupClick(g.typeKey)"` 下钻才渲染卡片列表。
当时的深链只把人送到页面上，卡列表一帧都没出现过。

## 本批产物能证明 / 不能证明

| 文件 | 能证明 | 不能证明 |
| --- | --- | --- |
| `before-profile.png` | 「我的」页头部蓝色渐变，作页身份对照基线 | — |
| `wrong-questions.png` | **目标页已到达**：头部纯白 + `#333333` 标题，与 `wrong-questions.uvue` 的 header 特征一致（截图为 `cli launch app-android --pagePath pages/profile/wrong-questions` 之后所摄） | 页面处于**分组视图**；卡片列表未渲染 |
| `card-dex-strings.txt` | 被验树确实是本次改动：设备包 `wrong-question-card/classes.dex` 解出 16 条中文串，含新增的 `最近` / `我上次选的答案：` / `正确答案：` / `解析：` / `暂无解析` / `（无作答记录）`，且 `record-question-img` class 名在内 | 这些元素**在屏幕上的呈现**（一帧都没出现） |

## 结论

**①a 未通过，且未主张任何一项展示正确。** 四条验收展示（题干图 / 答案与解析折叠区 /
我上次选的答案 / 最近错误时间）在本批产物里**一条都没有被观察到**。

阻塞原因两条，第二条是本文件首版漏掉的**真因**：

1. **视图不对**（主因）：错题本默认分组视图，没下钻就看不到卡片 —— 这是本批取证失败的真正原因，
   与「能不能读屏」无关。
2. **本会话读不了屏**（次要；对**文字类**判据仍成立）：视觉服务配额耗尽；本机 Windows OCR 在当前
   shell 起不来（`Operation is not supported on this platform`）；`uiautomator dump` 只有底部 tab bar
   四个文本节点（uniapp-x 原生渲染不上无障碍树）。

**但结构类判据不需要读屏** —— 先例 `docs/verification/device/1080/`（PR #1092）用的就是
`structure-check.py`（本地 PIL + numpy，按颜色 / 尺寸 / 条数断言像素结构），不是 OCR。

## 重跑时怎么做（最短路径）

1. 深链进 `pages/profile/wrong-questions`（默认分组视图）。
2. **先下钻**：点一个分组行 —— 分组行 `.group-icon` 在本机固定在 `x≈76-149`，三行行心实测
   `y≈885 / 1093 / 1298`；点行内任意位置（如 `x≈400, y≈885`）即进列表视图。
3. 再按先例写 `structure-check.py` 断言：右半区出现操作行（`#2979ff` 实心 + `#f44336` 描边各 ≥1 处/卡）；
   点「查看答案与解析」后卡片高度增长，且新增一块 `#f8f9fa` 面板内含 `#666666` 文字行。
4. 零占位判据：比对「有图卡」与「无图卡」高度差是否恰为图片块高度（`height:320rpx` = 493 px）。
5. 文字类判据（如「最近 YYYY-MM-DD HH:mm」是否有值）若需主张，另用能读屏的会话，否则明确写成未主张。
