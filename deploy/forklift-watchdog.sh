#!/usr/bin/env bash
# 生产链路存活巡检（#1277）
#
# 为什么存在：2026-09-22 pve-01 因内核升级重启，LXC 101 没配 onboot ⇒ 外部诊断助手静默停摆
# 约 4 小时。后端对助手故障只回友好中文文案（ADR-0032「不做熔断」的裁决不变），学员看得到
# 提示、没人看得到故障。本脚本把「没人知道」变成「6 分钟内有一封邮件」。
#
# 装法（pve-02，生产应用宿主）：
#   install -m 755 deploy/forklift-watchdog.sh /usr/local/bin/forklift-watchdog.sh
#   install -m 644 deploy/systemd/forklift-watchdog.{service,timer} \
#       deploy/systemd/forklift-alert@.service /etc/systemd/system/
#   printf '%s\n' 'WATCH_TO_EMAIL=you@example.com' > /etc/forklift-watchdog.env  # 再补 SMTP_*
#   chmod 600 /etc/forklift-watchdog.env
#   systemctl daemon-reload && systemctl enable --now forklift-watchdog.timer
# 自检（不发消息、不改状态）：/usr/local/bin/forklift-watchdog.sh --dry-run
# 发信自检（真发一封、不改状态）：/usr/local/bin/forklift-watchdog.sh --test-mail  ← 装完必跑
#
# 装完之后不用再靠人记得重装：`install_drift` 这一面会把「跑着的副本」与
# `$DEPLOY_PATH/deploy/forklift-watchdog.sh`（CD 每次部署都会刷新）逐字节比对，不一致就出声。
# 2026-11-04（#2270）：这条是补上 #2127 剩下的半边——上传解决了「拿不到新文件」，
# 没解决「新文件不生效」。redis 假告警（2026-10-29）与诊断端点假告警（2026-11-04）都是它。
#
# 配置（/etc/forklift-watchdog.env，root 600）：
#   WATCH_TO_EMAIL   收件人，逗号分隔可多个
#   SMTP_HOST/SMTP_PORT/SMTP_USERNAME/SMTP_PASSWORD/SMTP_FROM  与部署 .env 同一套凭据的副本
#   可选覆盖：WATCH_FAIL_THRESHOLD(3) WATCH_COOLDOWN_MIN(60) WATCH_DISK_CRIT_PCT(85)
#             WATCH_ASSISTANT_HEALTH_URL WATCH_PUBLIC_API_URL WATCH_FE_HOST
#             WATCH_REMOTE_HOST WATCH_REMOTE_LXC WATCH_SCRIPT_REF
#
#   三个面默认「不适用」而不是「故障」：
#     assistant_health  需要 WATCH_ASSISTANT_HEALTH_URL；留空 = v2 未配置，合法状态，不报警
#     disk_lxc          需要 WATCH_REMOTE_HOST + WATCH_REMOTE_LXC；该机已无流量、保留待清理
#     install_drift     需要 $DEPLOY_PATH/deploy/ 下存在参考副本（部署宿主上才有）
#   「不适用」在 --dry-run 里逐面打「跳过」，不会混进告警数里。
#
# 发信失败 = 本单元失败（timer 的 OnFailure= 会再走一封），绝不静默跳过。
# 已知边界：env 文件本身缺失/缺必需键时，兜底投递器用的是同一个文件 ⇒ 也发不出，只剩 journald。
set -euo pipefail

TAG=forklift-watchdog
log() { echo "[$TAG] $*"; }
die_no_mail() { log "致命：$*"; exit 1; }

# 先 source 再解析默认值：顺序反了会让 env 文件里的 WATCH_* 覆盖全部静默失效
# （`${VAR:-默认}` 在 source 之前求值就取不到 env 文件的值）。演练 C 实测踩到。
ENV_FILE=${ENV_FILE:-/etc/forklift-watchdog.env}
[ -r "$ENV_FILE" ] || die_no_mail "读不到 $ENV_FILE（发信与收件配置是硬前提）"
# set -a 是必需的而非风格：. 只设 shell 变量，send_mail 的 python 子进程读的是 os.environ。
# 2026-09-22 演练 A 实测：漏掉 export ⇒ 每条告警 KeyError: 'SMTP_USERNAME'、监控自己瞎了。
set -a
# shellcheck source=/dev/null
. "$ENV_FILE"
set +a
: "${WATCH_TO_EMAIL:?WATCH_TO_EMAIL 未设置}"
: "${SMTP_HOST:?SMTP_HOST 未设置}"
: "${SMTP_USERNAME:?SMTP_USERNAME 未设置}"
: "${SMTP_PASSWORD:?SMTP_PASSWORD 未设置}"

STATE_FILE=${STATE_FILE:-/var/lib/forklift-watchdog/state}
FAIL_THRESHOLD=${WATCH_FAIL_THRESHOLD:-3}
COOLDOWN_S=$(( ${WATCH_COOLDOWN_MIN:-60} * 60 ))
DISK_CRIT_PCT=${WATCH_DISK_CRIT_PCT:-85}
# ---- 三个可空面：留空 = 不适用，不是故障（#2270）----
# assistant_health：v2 的健康检查要 DIAGNOSIS_ASSISTANT_URL + X-API-Key，而按 #2237 的口径
# 「未配置」是合法状态（功能未启用），不该每 6 分钟发一封信。所以这里**没有默认值**：
# 部署宿主上把 WATCH_ASSISTANT_HEALTH_URL 写进 env 文件才启用这一面。
# 旧 lxc101 的 http://172.17.1.23:8000/assistant/api/health 探的是已下线的 v1 通道
# （机器还在跑，但近 7 天零请求，见 #2243 勘察），写死它就是留一个已退役面。
ASSISTANT_HEALTH_URL=${WATCH_ASSISTANT_HEALTH_URL:-}
# public_api：探「后端活没活」，与诊断功能解耦。原 /diagnosis/brands 随 PR #2265 删除；
# 换成 /api/ai-assistant/models——它仍在、仍走统一信封，且不依赖对方 OSS/上游可用性
# （配图链路是外部依赖，不该让外部抖动把整个后端判红）。
PUBLIC_API_URL=${WATCH_PUBLIC_API_URL:-https://api.gdyidao.cn/api/ai-assistant/models}
# 反代探活的目标主机（#1760）：必须是**站群里的真实 Host**，不再是 localhost。
# 留 WATCH_FE_HOST 覆盖口，是为了能在不打生产的前提下演练「探针失败必须变红」。
FE_HOST=${WATCH_FE_HOST:-api.gdyidao.cn}
# disk_lxc：默认两段都清空 ⇒ 该面不适用。
# 理由不是「机器已退役」——lxc101 仍在 pve-01 上运行（2026-11-04 上机实测 pct list 为 running），
# 而是「已无流量、保留待清理」：近 7 天 /ai-assistant/chat 零请求。上机前以为它已下线是错的。
# 将来若真有机型要跨机看水位，在 env 文件里同时给 host 与 lxc 号即可复活，不必改脚本。
REMOTE_HOST=${WATCH_REMOTE_HOST:-}
REMOTE_LXC=${WATCH_REMOTE_LXC:-}
EXPECT_CONTAINERS=(forklift-frontend-prod forklift-backend-prod forklift-pg-prod forklift-libreoffice-prod)
# install_drift：跑着的这份脚本 vs CD 上传到部署目录的参考副本。
# SELF_PATH 用 $0 的真实路径（systemd 跑的是 /usr/local/bin/forklift-watchdog.sh）。
SELF_PATH=${WATCH_SELF_PATH:-$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || printf '%s' "$0")}
SCRIPT_REF=${WATCH_SCRIPT_REF:-${DEPLOY_PATH:-/opt/forklift-training}/deploy/forklift-watchdog.sh}

MODE=check
DRY_RUN=0
FAILED_UNIT=unknown
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  --test-mail) MODE=test-mail ;;
  --alert-failure) MODE=alert-failure; FAILED_UNIT=${2:-unknown} ;;
esac

MAIL_FAILED=0

send_mail() { # send_mail <subject> <body>
  [ "$DRY_RUN" = 1 ] && { log "DRY-RUN 邮件：$1"; return 0; }
  # 发信失败不中断本轮：其余探测面仍要跑完（否则一个坏 SMTP 会让后面的面永远不被评估），
  # 但会以非 0 退出收尾 ⇒ timer 的 OnFailure= 仍会出声。
  if ! WATCH_TO_EMAIL="$WATCH_TO_EMAIL" SMTP_SUBJECT="$1" SMTP_BODY="$2" python3 - <<'PY'
import os, smtplib, ssl
from email.message import EmailMessage
to = [a.strip() for a in os.environ["WATCH_TO_EMAIL"].split(",") if a.strip()]
msg = EmailMessage()
msg["Subject"] = os.environ["SMTP_SUBJECT"]
msg["From"] = os.environ.get("SMTP_FROM") or os.environ["SMTP_USERNAME"]
msg["To"] = ", ".join(to)
msg.set_content(os.environ["SMTP_BODY"])
host, port = os.environ["SMTP_HOST"], int(os.environ.get("SMTP_PORT", "465"))
user, pw = os.environ["SMTP_USERNAME"], os.environ["SMTP_PASSWORD"]
if port == 587:
    s = smtplib.SMTP(host, port, timeout=30); s.starttls(context=ssl.create_default_context())
else:
    s = smtplib.SMTP_SSL(host, port, timeout=30, context=ssl.create_default_context())
with s:
    s.login(user, pw)
    s.send_message(msg)
PY
  then
    MAIL_FAILED=1
    log "发信失败（不中断本轮，收尾以非 0 退出触发 OnFailure）：$1"
    return 1
  fi
}

# ---- 探测面：每个 probe_* 的三种返回 ----
#   0 = 正常；1 = 故障（原因写进全局 REASON）；2 = 不适用（未配置/无参考副本，同样把原因写进 REASON）
# 三态而不是两态是 #2270 的要点：「v2 没配」和「v2 配了但连不上」必须分开，前者是合法状态。
REASON=""

probe_assistant_health() {
  local body
  if [ -z "$ASSISTANT_HEALTH_URL" ]; then
    REASON="未配置 WATCH_ASSISTANT_HEALTH_URL（诊断功能未启用即合法状态）"
    return 2
  fi
  body=$(curl -fsS -m 8 "$ASSISTANT_HEALTH_URL" 2>&1) || { REASON="诊断服务 health 不可达（$ASSISTANT_HEALTH_URL）：$body"; return 1; }
  # v2 的 /api/health 回 {"ok":true,...}；ok 为 false 时上游是可达但不可用，仍算故障。
  grep -q '"ok":true' <<<"$body" || { REASON="诊断服务 health 非 ok：${body:0:200}"; return 1; }
}

probe_containers() {
  local ps line name state bad=()
  ps=$(docker ps -a --format '{{.Names}}|{{.Status}}' 2>/dev/null) || { REASON="docker ps 失败"; return 1; }
  for name in "${EXPECT_CONTAINERS[@]}"; do
    line=$(grep -m1 "^${name}|" <<<"$ps" || true)
    state=${line#*|}
    if [ -z "$line" ]; then bad+=("$name 不存在")
    elif [[ $state == unhealthy* ]]; then bad+=("$name unhealthy")
    # docker pause 后状态是「Up … (Paused)」：进程冻结、请求全挂，但 healthcheck 要等
    # interval×retries 才翻转 ⇒ 只认 unhealthy 会漏掉整段故障窗口（演练 B 实测）。
    elif [[ $state == *Paused* ]]; then bad+=("$name paused")
    elif [[ $state == Restarting* || $state == Exited* || $state == Created* || -z $state ]]; then bad+=("$name ${state% *}")
    fi
  done
  [ ${#bad[@]} -eq 0 ] || { REASON="容器异常：${bad[*]}"; return 1; }
}

# 公网 «后端活着» 判据。原 /diagnosis/brands 随 PR #2265 删除（#2242 之后同形的引导面是
# /diagnosis/intents，但它要 v2 上游 → 上游没配时 502，会让「后端活没活」被外部依赖绑架）。
# 所以选 /api/ai-assistant/models：不必登录、恒走统一信封、只证明本站这一层。
probe_public_api() {
  local body
  if [ -z "$PUBLIC_API_URL" ]; then
    REASON="未配置 WATCH_PUBLIC_API_URL"
    return 2
  fi
  body=$(curl -fsS -m 12 "$PUBLIC_API_URL" 2>&1) || { REASON="公网后端端点不可达（$PUBLIC_API_URL）：$body"; return 1; }
  grep -qE '"code"[[:space:]]*:[[:space:]]*(0|200)' <<<"$body" || { REASON="公网后端端点信封异常：${body:0:200}"; return 1; }
}

# 前端「在跑」≠「在服务」：frontend 是 network_mode: host，compose 里 healthcheck 被显式
# disable（#496 血账：host 网络下容器内 wget 跟随 301 → 证书不匹配 → 误报 unhealthy）。
# 判据与 deploy/deploy.py 的宿主侧探活同口径（#483）：打 **api.<domain> 的 /health，只认 200**。
#
# 为什么不再打 https://localhost（#1760，2026-10-22 切换日实测）：
# 那是 nginx 时代的事实 —— 443 上有 default_server 兜任何 Host。反代换成 Caddy 后没有
# localhost 站点 ⇒ SNI 无证书 ⇒ 握手失败 ⇒ 恒 000 ⇒ **每轮巡检必报假警**，把真告警盖住。
# --resolve 钉回环：不依赖 DNS（测试机/换域时域名不一定指回本机）；-k 保留：内部自签模式也要过。
probe_frontend_health() {
  local code
  code=$(curl -sk -o /dev/null -w '%{http_code}' -m 8 \
    --resolve "${FE_HOST}:443:127.0.0.1" "https://${FE_HOST}/health" 2>/dev/null || echo 000)
  [ "$code" = "200" ] || { REASON="前端反代未在服务：https://${FE_HOST}/health 非 200（本机回环 --resolve，-k 跳证书）"; return 1; }
}

disk_pct_of() { # disk_pct_of <usage%> -> echoes int
  local raw=${1%\%}; echo "${raw//[^0-9]/}"
}

probe_disk_host() {
  local pct
  pct=$(df -P / | awk 'NR==2{print $5}') || { REASON="本机 df 失败"; return 1; }
  pct=$(disk_pct_of "$pct")
  [ "$pct" -lt "$DISK_CRIT_PCT" ] || { REASON="pve-02 根盘已用 ${pct}%（阈值 ${DISK_CRIT_PCT}%）"; return 1; }
}

probe_disk_lxc() {
  local out pct
  if [ -z "$REMOTE_HOST" ] || [ -z "$REMOTE_LXC" ]; then
    REASON="未配置 WATCH_REMOTE_HOST / WATCH_REMOTE_LXC（lxc101 已无流量、保留待清理）"
    return 2
  fi
  out=$(ssh -o BatchMode=yes -o ConnectTimeout=8 "$REMOTE_HOST" \
          "pct exec $REMOTE_LXC -- df -P /" 2>&1) || { REASON="跨机取 LXC$REMOTE_LXC df 失败：${out:0:120}"; return 1; }
  pct=$(awk 'NR==2{print $5}' <<<"$out")
  [ -n "$pct" ] || { REASON="LXC$REMOTE_LXC df 输出无水位列"; return 1; }
  pct=$(disk_pct_of "$pct")
  [ "$pct" -lt "$DISK_CRIT_PCT" ] || { REASON="LXC$REMOTE_LXC 根盘已用 ${pct}%（阈值 ${DISK_CRIT_PCT}%）"; return 1; }
}

# 安装漂移（#2270 选 B）：CD 每次部署都会把仓库里的脚本刷到 $DEPLOY_PATH/deploy/，
# 而 /usr/local/bin 里那份只有人工 install 才会换。两者不一致 ⇒ 「跑着的判据不是仓库的判据」，
# 这正是 redis 假告警（2026-10-29）与诊断端点假告警（2026-11-04）的共同成因。
# 判据用 sha256 逐字节比：比 mtime 稳（上传会刷新 mtime，内容却可能没变 → 假红）。
probe_install_drift() {
  local self_sum ref_sum
  if [ ! -r "$SCRIPT_REF" ]; then
    REASON="部署目录里没有参考副本（$SCRIPT_REF）：本站不是部署宿主，该面不适用"
    return 2
  fi
  if [ ! -r "$SELF_PATH" ]; then
    REASON="读不到自身路径（$SELF_PATH）：漂移判据失去输入"
    return 1
  fi
  self_sum=$(sha256sum "$SELF_PATH" 2>/dev/null | awk '{print $1}')
  ref_sum=$(sha256sum "$SCRIPT_REF" 2>/dev/null | awk '{print $1}')
  [ -n "$self_sum" ] && [ -n "$ref_sum" ] || { REASON="sha256sum 取不到摘要"; return 1; }
  [ "$self_sum" = "$ref_sum" ] || {
    REASON="巡检脚本与仓库不一致：跑着的 $SELF_PATH ($self_sum) ≠ $SCRIPT_REF ($ref_sum)；按 deploy/README.md「巡检脚本与 systemd 单元」重装一次"
    return 1
  }
}

# ---- 状态与告警 ----
mkdir -p "$(dirname "$STATE_FILE")"
[ -f "$STATE_FILE" ] || : > "$STATE_FILE"

state_get() { awk -v k="$1" '$1==k{print $2, $3, $4}' "$STATE_FILE"; }
state_set() {
  [ "$DRY_RUN" = 1 ] && return 0   # --dry-run 只读：不写状态、不发信
  local k=$1 fails=$2 alerted=$3 last=$4 tmp
  tmp=$(mktemp "$STATE_FILE.XXXX")
  awk -v k="$k" '$1!=k' "$STATE_FILE" > "$tmp"
  printf '%s %s %s %s\n' "$k" "$fails" "$alerted" "$last" >> "$tmp"
  mv "$tmp" "$STATE_FILE"
}

run_check() {
  local -A probe_fn=(
    [assistant_health]=probe_assistant_health
    [containers]=probe_containers
    [public_api]=probe_public_api
    [frontend_health]=probe_frontend_health
    [disk_host]=probe_disk_host
    [disk_lxc]=probe_disk_lxc
    [install_drift]=probe_install_drift
  )
  local order=(assistant_health containers public_api frontend_health disk_host disk_lxc install_drift)
  local now=$(date +%s) down=() skipped=() name fails alerted last rc
  for name in "${order[@]}"; do
    REASON=""
    # 三态：2 = 不适用。绝不用「返回 2 也算过」这种含糊写法——它会把故障也吞掉。
    rc=0
    "${probe_fn[$name]}" || rc=$?
    if [ "$rc" = 2 ]; then
      log "跳过：$name（$REASON）"
      skipped+=("$name")
      # 不适用期间把该面的累计清零：否则「昨天真故障 2 次 + 今天不适用」会在恢复该面时
      # 直接跨过阈值（1 次真故障就当第 3 次发信）。
      read -r fails _ _ <<<"$(state_get "$name" || echo '0 0 0')"
      [ "${fails:-0}" = 0 ] || state_set "$name" 0 0 0
      continue
    fi
    if [ "$rc" = 0 ]; then
      read -r fails alerted last <<<"$(state_get "$name" || echo '0 0 0')"
      fails=${fails:-0}; alerted=${alerted:-0}; last=${last:-0}
      if [ "$alerted" = 1 ]; then
        send_mail "【已恢复】叉车生产巡检：$name" "$(hostname) 上 $name 恢复正常（连续 $fails 次失败后转好）"
        log "恢复：$name"
        state_set "$name" 0 0 0
      elif [ "$fails" != 0 ]; then
        state_set "$name" 0 0 0
      fi
    else
      read -r fails alerted last <<<"$(state_get "$name" || echo '0 0 0')"
      fails=$(( ${fails:-0} + 1 )); alerted=${alerted:-0}; last=${last:-0}
      log "失败：$name（连续 $fails 次）$REASON"
      state_set "$name" "$fails" "$alerted" "$last"
      if [ "$fails" -ge "$FAIL_THRESHOLD" ]; then
        if [ "$alerted" = 1 ] && [ $(( now - last )) -lt "$COOLDOWN_S" ]; then
          log "跳过告警（冷却中）：$name"
        else
          if send_mail "【告警】叉车生产巡检：$name" \
"$(hostname) 上 $name 连续 ${fails} 次失败。

原因：$REASON

判据来源：/usr/local/bin/forklift-watchdog.sh --dry-run
状态文件：$STATE_FILE
journald：journalctl -u forklift-watchdog.service --since '-30 min'
探测面：诊断服务 health（未配置则跳过）/ pve-02 生产容器 / 公网后端 $PUBLIC_API_URL /
       前端反代 $FE_HOST/health / 本机磁盘水位 / LXC 磁盘水位（未配置则跳过）/
       巡检脚本与仓库副本是否一致"
          then
            log "已告警：$name"
            state_set "$name" "$fails" 1 "$now"
          else
            # 没投出去就不置 alerted、也不起冷却：下一轮继续重试，直到真的送达。
            state_set "$name" "$fails" 0 0
          fi
        fi
        down+=("$name")
      fi
    fi
  done
  log "巡检完成：达告警级别 ${#down[@]}/$(( ${#order[@]} - ${#skipped[@]} ))（不适用 ${#skipped[@]} 面：${skipped[*]:-无}；阈值：同一面连续 ${FAIL_THRESHOLD} 次）${down[*]:+ ⇒ ${down[*]}}"
}

case "$MODE" in
  test-mail)
    # --dry-run 不发信，抓不到发信路径上的 bug（演练 A 就是这么漏出 SMTP_* 未导出的）。
    # 这一条是唯一能证明「告警通道真的通」的自检。
    send_mail "【自检】叉车生产巡检发信通道正常：$(hostname)" \
"这是一封 forklift-watchdog 安装自检信，不代表任何故障。
收到即说明 SMTP 凭据、收件人与 send_mail 的变量传递三者都成立。"
    ;;
  alert-failure)
    send_mail "【告警】叉车生产巡检自身失败：$FAILED_UNIT" \
"$(hostname) 上 $FAILED_UNIT 执行失败 ⇒ 巡检本身可能已经瞎了（本次 4 小时盲区防的就是「静默失效」，包括监控自身）。

journalctl -u '$FAILED_UNIT' -n 50 --no-pager
journalctl -u forklift-watchdog.service -n 80 --no-pager"
    ;;
  *) run_check ;;
esac

if [ "$MAIL_FAILED" = 1 ]; then
  log "本轮存在投递失败的告警 ⇒ 以非 0 退出，交给 OnFailure= 追发"
  exit 1
fi
