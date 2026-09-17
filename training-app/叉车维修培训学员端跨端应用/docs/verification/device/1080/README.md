# #1080 ①a 真机取证记录（任务中心 growth 分组死代码清理）

## 被验对象

- 分支：`feat/mobile-1080-task-center-growth-cleanup`
- 被验树：`433fa991`（本 PR head）
- 设备：`b32d8398` / `23049RAD8C` / Android 15（USB；仅只读 `adb`）
- 页面：`pages/points/task-center`（走 `cli launch app-android --pagePath` 深链切页）

## 机检行

```
AUTO_SCREENSHOT_RESULT ok=True pages=1 skipped=0
导航已落定（247s，已进入目标页且画面稳定（相邻两次采样差异 0% ≤ 0.5%））
```

## 产物与判据

| 产物 | 判据 |
| --- | --- |
| `task-center.png` | `adb exec-out screencap -p`；1080×2400；sha256 `AFB0E504DB0F8DC4…` |
| `logcat-app.txt` | 应用自打的页面入口行 `进入页面:pages/points/task-center`（14:14:07，22 个 dom 元素、onReady 243ms） |
| `structure-check.py` / `structure-check.txt` | 像素结构判据（本地 PIL + numpy，不经外部服务） |

### 像素结构判据（`structure-check.txt` 可复现）

页面每段表头有一条 `.section-underline`（`#2979FF`、8rpx 高），每个任务行有一个 `.task-icon-wrap`
（`.icon-daily` = `#FFF3E0`、`.icon-newbie` = `#E8F5E9`，均 72rpx 见方）。据此在截图上数纵向色带：

- `#2979FF` 宽条（h=11px、x=46–137）= **恰好 2 条**（y=773 与 y=2090）⇒ 渲染出**两组**，且无第三段
- `#FFF3E0`（h=104px、x 自 70 起）= **6 条**（y=895 / 1081 / 1267 / 1453 / 1639 / 1825）⇒ 每日任务 **6 项**
- `#E8F5E9` = 1 条整 + 1 条被屏幕下缘截断 ⇒ 新手任务段**存在**且首行已渲染

## 未主张事项（写实）

1. **未目视核对图内文字**：本机无本地 OCR 引擎（无 `tesseract` / `easyocr` / `paddleocr` / `cv2`；
   WinRT `Windows.Media.Ocr` 在 pwsh 7 下加载失败），视觉服务返回配额错误
   （`accounts that have not been recharged can only try 10 times`）。⇒ 图内**文字内容**
   （各任务标题、「每日登录」的描述文案）**未经机器读取核对**，只由源码与迁移逐字确认。
2. **新手任务 4 项未在同一帧内见到**：该段落位于屏幕之外（页面总高 > 2400px）。「4 项」由**数据面**
   证据支撑（迁移 `000034` 把 `points_task_config.group` 收窄为 `('daily','newbie')`，后端契约
   `points_claim_contract_test.go` 的夹具与迁移后生产种子同形），**不由本帧断言**。
3. **未注入任何输入**：遵守 `device-capture.ps1` 的只读红线，本记录**未**使用 `adb shell input`，
   故无法下滑截取屏幕外内容。
4. **「无第三段」的排除依据是构造性的、不是像素性的**：`buildGroups` 只遍历 `order = ['daily','newbie']`，
   `rest` 兜底段仅在「分组不在 order 内」时出现，而 DB 的 CHECK 已不允许第三个分组值。
5. `classes.dm` 的 `ziparchive` W 级警告在**全应用约 110 个页面/组件**上各出现一次（含本页），
   属 HBuilderX 产物的既有现象，与本改动无关；app pid 上无 FATAL / Exception。

## 与基线的关系

`docs/verification/` 下**没有** task-center 的历史基线（`screenshot-diff` 首次运行不建立基线，
除非显式 `-UpdateBaseline`）。本轮**未**建立基线（避免拿本轮 after 图自证基线）。故本记录只主张
「该帧渲染出两组、每日 6 项、新手段存在」，**不主张**「与某历史版本逐像素一致」。
