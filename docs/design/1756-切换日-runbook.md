# 切换日 runbook（票 #1756 / ADR-0077）

> 一次原子操作：**反代换 Caddy + 域族切换 + 资源域回填 + 旧域退役**。顺序不可颠倒，任一步失败即停在原地（此时旧域仍在服务）。
> 前置判据不满足**任何一条**都不要开始。

## 0. 前置（全部为「是」才继续）

| # | 判据 | 验法 |
| --- | --- | --- |
| 1 | 新域备案生效 | `curl -sI http://www.gdyidao.cn/` 不再 302 到 `dnspod.qcloud.com/static/webblock.html` |
| 2 | 全部子域解析到生产机 | `api/training/mentor/manage/recruit/valuation/cdn/dify + 裸域 + www` 逐个 `Resolve-DnsName` ⇒ 183.36.195.104 |
| 3 | 证书策略不需要人工证书 | GitHub secrets 里 `SSL_FULLCHAIN`/`SSL_PRIVKEY` 待删；`TLS_MODE` 由 CD 按环境注入 |
| 4 | 门户回源目标已确定 | EdgeOne 控制台的 API 地址将改为 `https://api.<新域>`；`EDGE_ORIGIN_TOKEN` 两处同值 |
| 5 | 微信后台域名已加 | 小程序 request 合法域名 / 业务域名；公众号 JS 安全域名 + 网页授权回调（App 与小程序**均未发布**，故不需过渡期） |
| 6 | PR #1757 CI 绿 | `ci-summary = success`（分支 tip） |

## 1. 回填对象存储 URL（**必须在切换前完成**）

判据是**路径前缀**、与域名无关，所以这次回填安全；但顺序错就是「切完图片全碎」。

```bash
# 1) dry-run：只报每表命中行数，不落库
docker compose -f docker-compose.prod.yml run --rm backend \
  /app/bin/backfill-asset-domain -from https://cdn.gccsmile.com/ -to https://cdn.gdyidao.cn/

# 2) 把上一步的行数抄进 PR #1757 的评论（记账，事后可比对）

# 3) 落库（整轮一个事务）
docker compose -f docker-compose.prod.yml run --rm backend \
  /app/bin/backfill-asset-domain -from https://cdn.gccsmile.com/ -to https://cdn.gdyidao.cn/ -write

# 4) 幂等自证：第二次必须命中 0 行
docker compose -f docker-compose.prod.yml run --rm backend \
  /app/bin/backfill-asset-domain -from https://cdn.gccsmile.com/ -to https://cdn.gdyidao.cn/
```

⚠️ 若第 4 步仍有命中，**停下**：说明中间被打断或有写回，先查清再切。

## 2. 切 secrets（**作用域坑已实测，照这张表改**）

2026-10-22 实测的 secret 分布（`gh secret list`）：

| secret | 作用域 | 动作 | 值 |
| --- | --- | --- | --- |
| `DOMAIN` | **repo** | 改 | `gdyidao.cn` |
| `AUTH_COOKIE_DOMAIN` | **repo** | 改 | `gdyidao.cn`（不带前导点，与既有口径一致） |
| `CORS_ORIGINS` | **repo** | 改 | 六个 `https://*.gdyidao.cn` 源 |
| `SSL_FULLCHAIN` / `SSL_PRIVKEY` | **repo** | **删** | 已无引用（Caddy 自签/自续） |
| `R2_PUBLIC_DOMAIN` | **repo + production + testing 三处都有** | **三处都要改** | `https://cdn.gdyidao.cn` |

⚠️ **`R2_PUBLIC_DOMAIN` 是这张表里唯一会咬人的一行**：它同时存在于仓库级与两个 environment 级，而 GitHub **environment 级优先**。只改仓库级等于没改——部署仍会用 environment 里的旧域，表现为「图片 URL 还是旧域」，且当时看不出报错。

另：仓库 Variables 里的 `API_URL` / `FRONTEND_URL` 若存在则同步改（只用于部署报告展示）。

## 3. 合并 PR #1757 → CD 自动上生产

合并后 `cd.yml` 的 gate 校验 PR 的 `ci-summary` 与 testing 冒烟，随后部署。**部署即换反代**。

## 4. 验证矩阵（逐条实测，不靠「部署成功」）

```bash
# 证书：每台主机都必须是 Let's Encrypt 签给本机名的证书
for h in api training mentor manage recruit valuation cdn dify; do
  echo | openssl s_client -connect $h.gdyidao.cn:443 -servername $h.gdyidao.cn 2>/dev/null \
    | openssl x509 -noout -subject -dates
done

# 后端与反代
curl -sf https://api.gdyidao.cn/health          # ok
curl -sI https://api.gdyidao.cn/api/health      # 200

# 四端登录页可达 + 页脚有主体与备案号（肉眼一次，抓 HTML 里 grep 一遍）
for h in training mentor manage recruit; do curl -sI https://$h.gdyidao.cn/ | head -1; done
curl -s https://training.gdyidao.cn/ | grep -o '粤ICP备2026005936号-3'

# 资源域：**三条必须成对验**（2026-10-22 事故的教训）
# 只验「不存在的对象 404」= 只验了兜底，路由整段坏掉也能绿。
N=avatars/<库里真实存在的一条 key>
curl -s  -o /dev/null -w '%{http_code}\n' "https://cdn.gdyidao.cn/$N"                    # ① 200（存在必须取得到）
curl -sI "https://cdn.gdyidao.cn/$N" | grep -i 'content-disposition\|x-content-type'      # ② 投递分档头 + nosniff
curl -s  -o /dev/null -w '%{http_code}\n' https://cdn.gdyidao.cn/not-a-prefix/x.png       # ③ 404，且正文是本站 "Not Found"（不是 RGW 的 NoSuchKey）
curl -s  -o /dev/null -w '%{http_code}\n' -X POST https://cdn.gdyidao.cn/avatars/x.webp   # ④ 403（写方法拦截）

# 浏览器侧：登录一次，确认 Cookie 域是 gdyidao.cn、且跨子域跳转不掉登录
```

**移动端**：改 `config/env.uts` 与 `api/credential.uts` 两处基址到 `https://api.gdyidao.cn/api`（这一步由移动端负责人发版，本 runbook 只记录判据）。

## 5. 门户回源切到新入口

EdgeOne 控制台：门户项目的 API 回源地址 `https://www.gccsmile.com` → `https://api.gdyidao.cn`；`NUXT_API_ORIGIN_TOKEN` 与后端 `EDGE_ORIGIN_TOKEN` 同值。切完验：门户内容精选页能拉到数据、图片走 `cdn.gdyidao.cn`。

**怎么判定「回源真的切过去了」**（2026-10-22 实测口径）：旧域 DNS 删除后，若门户 `/api` 仍返回 200，则回源必然已是新入口 —— 这是不依赖控制台的一条判据。切换瞬间可能看到 1～2 次 502（EdgeOne 回源重试），重试后恢复属正常。

⚠️ 切换日曾为「门户还打旧 Host」留过一个 **api-only 过渡块**（`www.gccsmile.com`）。回源确认切走后**必须删掉它**（`TestCaddyfileHostMatrixLock` 会挡住残留）。

## 6. 旧域退役

- DNS：删除 `gccsmile.com` 全部记录（含泛解析）。
- 证书：旧泛域证书（2027-01-10 到期）不再使用，也不再续期。
- 代码：`gccsmile` 在仓库内的残留只允许出现在**历史文档/ADR**里；`grep -r gccsmile --include=*.go --include=*.ts --include=*.conf` 应为空。

## 7. 回滚

| 现象 | 判据 | 处置 |
| --- | --- | --- |
| 证书签不下来 | `openssl s_client` 报的是自签或旧证书 | **不合并**（此时尚未切换）；已合并则该域不可用，按 `deploy-remote.sh --rollback` 回上一镜像 |
| 四端登录循环 | 登录后又被守卫打回登录页 | 查 `AUTH_COOKIE_DOMAIN` 与 `CORS_ORIGINS` 是否漏改；必要时把 `DOMAIN` 临时改回旧值**并重新部署**（旧域此时仍在） |
| 图片全碎 | `cdn` 返回 404 且对象存在 | 回填没做完就切了 ⇒ 重跑第 1 步（幂等），修完再验 |
| 后端拿不到真实 IP | 访问日志里 IP 是边缘节点 | 查 `EDGE_ORIGIN_TOKEN` 是否为空（入口应 fail closed 拒启）与回源头是否命中 |

## 附：这次切换的「为什么」

决策与理由见 `docs/adr/ADR-0077-域名与主体迁移的双域族.md`；别在这里复述，两处会漂。
