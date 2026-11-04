# 诊断切流 runbook（票 #2240）

> 把 `fault_diagnosis` 从「未配置」翻到外部 v2 通道，并证明能翻回来。
>
> **本文与票面步骤有一处实质性分歧，写在 §0 末尾**：票面假设存在 `ecs / legacy` 上游开关，
> 但该开关在 #2237 时随 v1 通道一起被硬删了。回滚不是"拨回 legacy"，而是"清空两个 secret 回到未配置"。
> 这个区别决定了 §6 的整段写法。

---

## 0. 前置（全部为「是」才继续）

| # | 判据 | 验法 | 现状（2026-11-04 17:30，真实联调后） |
| --- | --- | --- | --- |
| 1 | 对方 v2 已部署且 `/api/v2/*` 可达 | `curl -s -o /dev/null -w '%{http_code}' <基址>/api/v2/intents` ⇒ `200` | ✅ **200**。注意基址（见 §1.2）与健康检查路径 |
| 2 | 已拿到 32 位明文 `X-API-Key` | 对方私发；只存 GitHub Secrets | ✅ 已签发（32 位，写入对方 `src\forklift\data_ecs\forklift.db`，只存指纹） |
| 3 | 我方出口 IP 已登记进对方白名单 | 对方回执；`183.36.195.104` | ✅ 已在册，限流 300 次/分钟 |
| 4 | 图片 OSS 前缀已确认 | 与 `DIAGNOSIS_V2_OSS_BASE` 默认值比对 | ✅ 回包图片在 `…/forklift/repair_img/`，与默认值逐字一致 |
| 5 | 代码已上线并含 v2 适配 | `git log --oneline -1 --stat \| grep diagnosis` | ✅ #2237 `28a9c474` / #2242 `403b52e7` / #2244 `97e4d3b2` / #2280 `37123d88` 均已合并 |
| 6 | 生产当前是「未配置」态（起点干净） | `docker exec forklift-backend-prod printenv \| grep DIAGNOSIS` ⇒ 两个都为空 | ✅ 实测 `DIAGNOSIS_ASSISTANT_URL` len=0、`DIAGNOSIS_V2_API_KEY` len=0 |
| 7 | 传输加密 | `<基址>` 的 scheme | ⚠️ **http 明文**。8080 不是 TLS、443 不通，Key 明文过公网 —— 遗留风险，见 §0.2 |

**任何一条为 ❌ 就不要开始；第 7 条是 ⚠️ 不是 ❌，按用户 2026-11-04 拍板「先按 http 接上做联调，同时要求对方上 TLS」推进。**

### 0.2 基址：v2 在 IP 上，不在 README 的域名上

`docs/reference/外部AI助手/README.md:3` 写的 `https://forklift-repair-assistant.app.workbuddy.host` 是 **v1 旧实例**，不是 v2。实测对照（2026-11-04 17:30）：

| 地址 | `/api/health` | `/api/v2/intents` | 结论 |
| --- | --- | --- | --- |
| `http://218.244.148.138:8080` | 200 `env=ecs` `port=8080` `data_dir=C:\forklift\data_ecs` | **200** | **真 v2，基址填这个** |
| `https://forklift-repair-assistant.app.workbuddy.host` | 200 `env=local` `port=3000` `data_dir=/workspace/data` | 404 | v1 旧实例 |

另注意：**v2 没有 `/api/v2/health`**（404），健康检查只有 `/api/health`（我方 `DiagnosisCatalogService.Health` 打的就是这个，无鉴权）。所以前置判据第 1 条的验法用 `/api/v2/intents`，别用 `/api/v2/health` —— 后者在任何情况下都 404，会把「已部署」误判成「未部署」。

`/api/health` 里有两个长得像的字段：`oss_base` 是 `…/create`（建会话用），`image_base` 才是 `…/forklift/repair_img`（回答配图基准）。白名单只认后者。

**遗留风险：传输是明文 http。** `https://218.244.148.138:8080` 报 `SSL routines::wrong version number`，443 端口不通 —— 也就是说 `X-API-Key` 会以明文过公网。它被 IP 白名单与 300 次/分钟限流挡了一层，但**不是加密**。用户 2026-11-04 拍板「先按 http 接上做联调，同时要求对方上 TLS」，所以这一条要跟着对外清单一起推，**别当成已解决**。对方上 TLS 后只需把 secret 换成 `https://…` 再部署一次，代码侧零改动（`validateDiagnosisV2` 只校验 OSS base 是 https，不校验基址）。

### 0.1 与票面的实质分歧：没有 `legacy` 可回退

票面第 1 步写「上游开关设为 ecs」、第 4 步写「把开关改回 legacy」、完成定义写「legacy 服务不下线，至少保留 30 天作为回滚通道」。

**这三处都建立在一个已经不存在的开关上。** 事实：

- `backend/internal/aiassistant/` 全域搜 `upstream` 共 11 处命中，**全部是 HTTP 语义**（`diagnosis_catalog.go:250-260` 解上游 health JSON、`diagnosis_proxy.go:183` 解析子路径、测试里的 `"upstream boom"`），**没有一处是「选通道」的开关**。
- `legacy` 关键字唯一的命中是 `backend/internal/aiassistant/config.go:433-444` 的 AI 模型配置回退（`ai_assistant` 多绑定兼容），与诊断通道无关。
- 切流前后**只有一条通道**：`diagnosis_adapter.go` 的 `callSSE` → `POST {base}/api/v2/chat`。
- v1 的四个只读端点与图片代理在 `28a9c474`（#2237）里被**物理删除**，不是被开关藏起来。
- 那台 legacy 机器（pve-01 lxc101 `172.17.1.23:8000`）原先仍在运行，跑的是 `/assistant/api/*`；但我方代码里**已经没有任何路径指向它**（`grep -r "172.17.1.23\|/assistant/api/" backend/` ⇒ 0 命中）。**它已于 2026-11-04 14:44 停机**：

  ```text
  pct status 101         ⇒ stopped
  pct list               ⇒ 101  stopped  forklift-assistant
  pct config 101         ⇒ onboot: 0（已关，pve-01 重启不会自动拉起）
  curl 172.17.1.23:8000  ⇒ rc=28（连接超时，端口不再响应）
  ```

  停机前取证（7 天窗口，`docker logs forklift-assistant --since 168h`，共 8,989 条访问）：
  非 health 请求只有 **11 条 `POST /assistant/api/chat`** 与 2 条 `fault-codes`，其余全是
  watchdog 的 health 探测。最后一次真实对话是 `2026-11-03T13:54:43Z`，此后到停机零业务请求。
  需要回捞旧数据时 `pct start 101` 即可（`onboot: 0` 只影响开机自启，不挡手动启动）
  —— 它现在不构成回滚通道，也不需要跑着占资源。

⇒ **回滚 = 清空两个 secret 并重启**，功能回到「502 未配置」的友好错误态。
这不是"降级到旧通道"，是"关掉这个功能"。票面「legacy 服务保留 30 天」那条已在 2026-11-04 按此改写（机器停机，回捞改为按需 `pct start`）。

---

## 1. 写入配置（两条 secret，一处作用域）

### 1.1 先确认作用域，别只改一处

`gh secret list` 看 `DIAGNOSIS_ASSISTANT_URL` 与 `DIAGNOSIS_V2_API_KEY` 存在哪些作用域。
**GitHub environment 级优先于 repo 级** —— 这是 #1756 切换日踩过的坑（`R2_PUBLIC_DOMAIN` 三处都要改）。
若只在 repo 级建，`cd.yml` 的 `production` 环境里读不到。

### 1.2 写入

```bash
# 基址：填 IP:port（v2 在这里，不在 README 的域名上 —— 见 §0.2）。
# 记得带 http:// 且不带尾斜杠以外的路径；协议是明文 http，见 §0.2 的遗留风险。
gh secret set DIAGNOSIS_ASSISTANT_URL --body 'http://218.244.148.138:8080'

# Key：从对方私发处复制，32 位。绝不写进仓库、绝不贴进工单
gh secret set DIAGNOSIS_V2_API_KEY --body '<32 位明文>'
```

⚠️ **不要手工改 `deploy/env.defaults` 或 `docker-compose.prod.yml`。**
这两个变量走 `cd.yml:487-488` 的 secrets 映射 + `cd.yml:706-707` 的 `printf` 写入，
`env.defaults` 里**故意没有**它们（§22 的循环覆盖不到，见 `cd.yml:486` 的注释）。
改模板等于引入第二处事实源，下次 CD 会打回原样。

### 1.3 触发部署

合一个空提交或直接 `gh workflow run cd.yml`。判据不是「部署成功」，是：

```bash
docker exec forklift-backend-prod printenv | grep -E '^DIAGNOSIS_(ASSISTANT_URL|V2_API_KEY|V2_OSS_BASE)='
# 期望：URL 与 KEY 非空、OSS_BASE 是 …/forklift/repair_img（或按 §0 第 4 条确认后的值）
```

⚠️ **配置自洽性会挡住启动**：`config.Validate()` 的 `validateDiagnosisV2`（`backend/internal/config/config.go:664`）在 **Key 非空**时要求
OSS base 必须 `https` + host 非空 + path 以 `/forklift/repair_img` 开头。若第 4 条确认下来前缀是 `/create`，
后端会**直接启动失败** —— 这是有意的 fail-closed，不是 bug。
处置见 §6.3 的「后端起不来」一行。

---

## 2. 端到端走查（逐条实测，不靠「部署成功」）

按票面六项展开成可执行判据。全部在浏览器 + `curl` 两个面各验一次。

### 2.1 健康探测

```bash
# 我方代理端点：恒 200，ok 表达真实状态
curl -s https://api.gdyidao.cn/api/ai-assistant/diagnosis/health
# 期望：{"code":200,"message":"success","data":{"ok":true,"app_configured":true}}
# 注意：ok=false 也算「端点活着」，要看 app_configured 与 ok 两个字段都为 true

# 确认只投影两个字段（不是把对方 20 个字段透出去）
curl -s https://api.gdyidao.cn/api/ai-assistant/diagnosis/health | grep -c 'key_fp8\|data_dir\|workflow_id_head'
# 期望：0
```

### 2.2 五个只读端点逐个通

```bash
B=https://api.gdyidao.cn/api/ai-assistant/diagnosis
for p in "intents" "systems?brand=zhongli" "systems?brand=linde" "faults?brand=zhongli" "code-hint?q=E41"; do
  printf '%-28s ' "$p"
  curl -s -o /dev/null -w '%{http_code} %{time_total}s\n' "$B/$p"
done
# 期望：全 200；缓存头只在 intents/systems/faults 三条上出现
curl -sI "$B/intents" | grep -i cache-control   # ⇒ public, max-age=300
curl -sI "$B/code-hint?q=E41" | grep -ci cache-control  # ⇒ 0
```

**语义判据（不能只看 200）：**

- `intents` 的 `brands[]` 里 `zhongli.picker === "system"`、`linde.picker === "model"` —— 若对方改了，前端第二层会走错分支（按系统渲染型号）。
- `faults?brand=linde` 的条目数应与对方 v1 `/api/faults` 一致；**六张硬编码诉求卡的名字必须在这份列表里能找到**（否则点进去的检索词是空召回）。逐条核对：

```bash
curl -s "$B/faults?brand=zhongli&second=<某个系统id>" | node -e "const j=JSON.parse(require('fs').readFileSync(0,'utf8'));console.log(j.data.items.map(i=>i.name).join(' | '))"
```

人工比对六张卡发出的**语料原话**（注意发的是后者、不是前者标签）：

| 卡片标签 | 实际发出去的检索词 |
| --- | --- |
| 🚨 仪表报警亮了 | 仪表报故障码 |
| 🔋 整车没电 | 整车无法启动 |
| 🛢️ 举升无力 | 举升无力 / 不能起升 |
| 🔌 充电异常 | 不能充电 / 充电异常 |
| 🚛 不能行走 | 不行走 / 无法行走 |
| 🛞 转向失灵 | 转向失灵 / 无转向 |

右列这六个串**必须能在这份 faults 列表里找到**（或能被 code_hint/faults 命中）。找不到的那张卡就是空召回，
记进票面 —— 这六串是对方 v1 语料的副本（`guide.js:84-89` + `guide.py:97-106`），v2 若改了语料就会对不上。

**2026-11-04 真实联调已验：六张卡全中。** `GET /api/v2/faults?brand=zhongli`（不带 `second`，即品牌根层）逐字返回：

```
仪表报故障码 | 整车无法启动 / 无法上电 | 不行走 / 无法行走 | 举升无力 / 不能起升 | 不能充电 / 充电异常 | 转向失灵 / 无转向
```

⚠️ 注意**带 `second` 时是另一份列表**（如 `second=controller` 回「不行走 / 无法行走 | 不举升 / 无法起升 | 无法转向 | 报驱动控制器故障码 | 仪表报故障码」）—— 六张卡走的是**品牌根层**那条路，比对时别选错。

- `code-hint?q=E41` 的 `status` 应是 `known`（v1 实测如此）。真实联调另验了 `q=E01` ⇒ `known`、`q=E`（太短）与 `q=XYZ`（非码）⇒ `none`，与契约五态一致。

### 2.3 未登录访客走完四步

**用无痕窗口**（不带 token），走：品牌卡 → 系统/型号 → 维修部位 → 确认 → 发送。

| 观测点 | 判据 |
| --- | --- |
| 欢迎区三件套 | 品牌卡 2 张（中力 / 林德）、诉求卡 6 张、FAQ 每条可点 |
| 提示条 | **不出现**（出现 = `health.ok=false`，说明上游没通） |
| 点品牌卡 | 向导自动打开、停在第二步、系统列表已拉到（这条曾因 watch 注册顺序坏过，见 #2244） |
| 林德路径 | 型号列表 > 15 条时出现搜索框 |
| 组句 | 第 4 步预览的文字含品牌名 + 型号/系统名 + 现象；**林德版不含「根因分析」** |
| 发送后 | 答案上方出现能力徽章（若对方给了 badges）、展开来源折叠头可见「耗时 N.N 秒」 |

### 2.4 带图提问

发一张**仪表报警照片**（要有可读的文字/数字，才验得出来多模态是否真的跑通）。

| 判据 | 怎么看 |
| --- | --- |
| 图片送达 | 答案里引用了图上的具体信息（故障码、字样），而不是泛泛而谈 |
| 压缩生效 | 浏览器 devtools 网络面板里，`/api/ai-assistant/upload-image` 那一段的请求体 ≈ 数百 KB 而非数 MB |
| 张数上限 | 第 4 张图被挡下，弹「最多上传 3 张图片」 |
| 单张超限 | 传一张 > 5MB 的，返回「图片单张不能超过 5MB，请压缩后重试」 |

### 2.5 正文图走我方代理

**这是本票最容易被「看起来正常」骗过去的一项** —— 对方返回的图片 URL 若不在白名单内，会被降级成 alt 文字，页面不报错、只是没图。

| 判据 | 验法 |
| --- | --- |
| 答案里确实有图 | 找一条**会出图**的提问（仪表/铭牌类） |
| 图不是我方直连 OSS | devtools 里图片请求指向 `api.gdyidao.cn/api/ai-assistant/diagnosis/ossimg/...`，**不是** `hrwai-guangzhou.oss-cn-beijing.aliyuncs.com` |
| 代理真取到了 | `curl -sI '<上面那个 ossimg URL>'` ⇒ `200` + `Cache-Control: public, max-age=86400` |
| 降级不算通 | 若页面上是「诊断配图」文字而非图片，去后端日志找 `白名单外` 那行，把 `host` 与 `url_head` 记进票面 |

```bash
docker logs forklift-backend-prod --since 30m 2>&1 | grep -i '白名单外\|ossimg'
```

### 2.6 触发一次限流

对方默认 60 次/分钟。**用测试账号**连续打 `code-hint`（每 250ms 一次防抖，手动打 60 次即可）：

```bash
for i in $(seq 1 70); do curl -s -o /dev/null -w '%{http_code} ' "$B/code-hint?q=E$i" ; done; echo
```

| 判据 | 期望 |
| --- | --- |
| 我方返回码 | `429`（不是 500，也不是把自己当成 502） |
| 文案 | 「诊断服务请求过于频繁，请稍后再试」 |
| 前端表现 | 弹出可读提示，不是白屏或控制台报错 |
| `Retry-After` | 若对方给了，我方**是否透传**——当前实现不透传，记录现状即可 |

### 2.7 终端用户视角无 4xx/5xx

```bash
# 后端访问日志里筛诊断相关路径的非 2xx
docker logs forklift-backend-prod --since 1h 2>&1 \
  | grep -E 'diagnosis|ai-assistant/chat' | grep -E '"status":(4|5)[0-9][0-9]' | head -20
# 期望：空。若有，逐条判是「正常拒绝」（如 429 演练）还是真故障
```

---

## 3. 历史兼容

票面第 3 步写「翻到 ecs 后打开一条旧会话，来源折叠仍显示 manual 形状且能下载」。

⚠️ **这一步的判据已被 #2237 改掉了**：旧会话的 `sources` 里确实还是 manual 形状（`<<IMAGE:/assistant/static/manual/…>>`），
但 `DiagnosisSources.vue` 自 `403b52e7` 起**只回放文字、剥掉标记、不再产出任何 URL** —— 这是有意的（上游资源已下线，渲染即坏图）。

**修正后的判据：**

| 观测点 | 期望 |
| --- | --- |
| 打开旧会话 | 不报错、不白屏 |
| 来源折叠头 | 仍显示「资料来源（N 条）」 |
| 展开后 | 能看到文字摘录与页码 |
| **不应出现** | 任何 `<img>`、任何 `/diagnosis/manual/` 链接、任何「打开 PDF 来源」按钮 |
| 控制台 | 无 404 刷屏（有 404 说明标记没剥干净） |

```bash
# 直接查库确认旧行的形状（只读）
docker exec forklift-pg-prod psql -U forklift -d forklift_training -c \
  "SELECT id, left(sources, 200) FROM ai_chat_messages WHERE sources LIKE '%<<IMAGE:%' ORDER BY id DESC LIMIT 3;"
```

**如果产品上认为「旧会话看不到图」不可接受**，那是另一张票的事（数据清理或渲染兜底），不在切流射程内 —— 记进遗留即可。

---

## 4. 观察窗口（至少一个工作日）

记录三类读数，落进 PR 正文或票面：

| 指标 | 采集方式 | 关注阈值 |
| --- | --- | --- |
| 错误率 | `docker logs ... \| grep diagnosis \| grep -c '"status":5'` ÷ 总请求数 | 任一条 5xx 都要归因 |
| 响应时延 | 上面 §2.2 的 `%{time_total}`；对方契约说纯文字 8~15 秒、带图 +10 秒 | 显著超出即查 `elapsed` 分布 |
| 限流命中 | 后端日志里 429 的条数 | > 0 就说明 60 次/分钟不够，去谈提额 |
| 降级次数 | 日志里 `白名单外` 的行数 | > 0 说明 OSS 前缀判断有偏，回到 §0 第 4 条 |

```bash
# 一条命令把四类读数拉齐
docker logs forklift-backend-prod --since 24h 2>&1 | grep -E 'diagnosis|ossimg' | \
  awk '{ if (/"status":5/) s5++; else if (/"status":429/) s429++; else if (/白名单外/) dg++ } END { print "5xx="s5+0, "429="s429+0, "降级="dg+0 }'
```

---

## 5. 文档更新

| 落点 | 改什么 |
| --- | --- |
| `CONTEXT.md:98` | 「承载方」从「外部诊断助手 v2（ECS 上的 FastAPI + bge-m3）」改为实际部署地址与形态 |
| `docs/adr/` 新编号 | 执行时取 `docs/adr/` 最大号 + 1。**2026-11-04 实测最大是 `ADR-0092-树身份-目录为入口与登记分支为身份.md`**，故届时为 `0093`；执行当天**重新确认**一次再命名，不预先写死 |
| `docs/adr/ADR-0032-…md` 状态行 | 已有「§决策1 与 §决策5 已随 v1 退役」的注记，若切流后有新事实（如实际基址）补一句 |
| `deploy/README.md` | 若 §1 发现了新的 secret 作用域坑，补进去 |

**ADR 必须回答的问题**（不是流程复述）：为什么没有回滚开关、v1 为什么物理删除而非开关化、以及这次切流的
「不可逆点」具体在哪一步。

---

## 6. 回滚（**真做一次，不是纸面承诺**）

### 6.1 回滚动作

```bash
# 1) 清空两个 secret（不回退到 legacy —— 没有 legacy 通道，见 §0.1）
gh secret delete DIAGNOSIS_ASSISTANT_URL
gh secret delete DIAGNOSIS_V2_API_KEY

# 2) 重新部署（CD 会用空值写 .env）
gh workflow run cd.yml

# 3) 验回滚生效
curl -s https://api.gdyidao.cn/api/ai-assistant/diagnosis/intents
# 期望：{"code":502,"message":"智能维修诊断服务未配置，请联系管理员"}
curl -s https://api.gdyidao.cn/api/ai-assistant/diagnosis/health
# 期望：{"code":200,...,"data":{"ok":false,"app_configured":false}}
```

**回滚要多久？** 一次 CD 部署的时间（实测约 3~5 分钟）。这个数字要写进票面 —— 它是「出事后多久能止损」的唯一答案。

### 6.2 回滚演练记录（必须填）

| 项 | 值 |
| --- | --- |
| 演练日期时间（含时区） | ← 执行时填 |
| 执行人 | ← 执行时填 |
| 起始状态 | ← 切流已完成且正常工作 |
| 从执行到 `/diagnosis/intents` 回到 502 的时长 | ← 实测填 |
| 期间用户可见影响 | ← 期望「诊断按钮点了报未配置」，其他功能不受影响 |

⚠️ **演练后要切回去**，并再验一次 §2.1。别把演练留在回滚态过夜。

### 6.3 故障处置表

| 现象 | 判据 | 处置 |
| --- | --- | --- |
| **后端起不来** | `docker logs forklift-backend-prod` 里有 `DIAGNOSIS_V2_OSS_BASE` 相关 `fmt.Errorf` | OSS 前缀与 `diagnosisOSSPathPrefix` 常量不符。两条路：① 按 §0 第 4 条改常量（`config.go:656`）并发一次；② 先清空 `DIAGNOSIS_V2_API_KEY` 回到未配置态止损（**注意：不能「只填 Key 不填 OSS base」—— Key 非空才会触发校验，两者必须成对**） |
| 全端点 502 但 health 200 | `public_api` 通、`diagnosis/intents` 502 | 基址错 / Key 错 / 对方鉴权拒。看后端日志的 `v2StatusError` 分支（401 会区分「Key 停用」与「IP 未授权」） |
| 403 且文案里有 IP | 对方 403 回显了 IP | 那是**对方看到的我方出口**，与 `183.36.195.104` 比对；不符则白名单登记错了 |
| 图全是 alt 文字 | 日志里 `白名单外` 有行 | OSS 前缀判断偏，回到 §0 第 4 条 |
| 429 频繁 | §4 的 429 计数偏高 | 去要提额；同时可以考虑前端把 `code-hint` 防抖拉长 |
| 答案里出现 `{{IMG:` 之类令牌 | 肉眼可见 | `done.answer` 的清洗规则与预期不符，把原文贴进票面找对方核 |

**基址填错时最常见的一种表现**：填成 `https://forklift-repair-assistant.app.workbuddy.host`（README 那个域名）。它 `/api/health` 也回 200、`app_configured` 也 true，所以**健康探测会骗过你**；但 `/api/v2/*` 全 404，于是引导面 502、对话 502。认这个坑的判据：`curl <基址>/api/v2/intents -H 'X-API-Key: …'` 回 404 而不是 200 —— 见 §0.2。

---

## 7. 完成定义

- [ ] §0 七条前置全绿（第 7 条是 ⚠️ 明文 http，按 §0.2 的处置推进）
- [ ] §1 两个 secret 写入且作用域正确、`printenv` 验到非空
- [ ] §2 七项走查逐条有读数（不是「试了一下没问题」）
- [ ] §3 历史会话回看符合修正后的判据
- [ ] §4 至少一个工作日的四类读数
- [ ] §5 词表与 ADR 已更新（ADR 编号届时取最大号 + 1）
- [ ] §6 回滚演练有明确时间戳、执行人、实测时长，且演练后已切回
- [ ] 证据按仓库口径写进 PR 正文：一行 `跑于 <sha7>` + 门读数行原样

---

## 附：为什么这份 runbook 与票面步骤不一致

票面写于 2026-11-03，当时 #2237 还没落地，`ecs / legacy` 双轨的假设成立。
`28a9c474`（2026-11-03）与 `403b52e7`、`97e4d3b2`（2026-11-04）三个提交之后：

1. v1 通道被物理删除，**不存在开关**；
2. 旧只读端点与图片代理全删，**旧会话的来源图与 PDF 链接不再可达**；
3. 图片改为白名单代理，**OSS 前缀成了硬约束**（不符即启动失败）。

所以本文在 §0.1 / §3 / §6 三处改写了票面的判据。**这三处已于 2026-11-04 回填到票面正文**
（每处标了「（改写）」并给理由），票面与本文现在一致。

唯一仍然只有本文有的是 §6.2 的回滚演练记录表 —— 它要等有环境时填实测时长。
