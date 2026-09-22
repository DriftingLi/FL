#!/usr/bin/env bash
# 生产链路存活巡检（#1277）
#
# 为什么存在：2026-09-22 pve-01 因内核升级重启，LXC 101 没配 onboot ⇒ 外部诊断助手静默停摆
# 约 4 小时。后端对助手故障只回友好中文文案（ADR-0032「不做熔断」的裁决不变），学员看得到
# 提示、没人看得到故障。本脚本把「没人知道」变成「6 分钟内有一封邮件」。
#
# 装法（pve-02，生产应用宿主）：
#   install -m 755 scripts/forklift-watchdog.sh /usr/local/bin/forklift-watchdog.sh
#   install -m 644 deploy/systemd/forklift-watchdog.{service,timer} \
#       deploy/systemd/forklift-alert@.service /etc/systemd/system/
#   printf '%s\n' 'WATCH_TO_EMAIL=you@example.com' > /etc/forklift-watchdog.env  # 再补 SMTP_*
#   chmod 600 /etc/forklift-watchdog.env
#   systemctl daemon-reload && systemctl enable --now forklift-watchdog.timer
# 自检（不发消息、不改状态）：/usr/local/bin/forklift-watchdog.sh --dry-run
#
# 配置（/etc/forklift-watchdog.env，root 600）：
#   WATCH_TO_EMAIL   收件人，逗号分隔可多个
#   SMTP_HOST/SMTP_PORT/SMTP_USERNAME/SMTP_PASSWORD/SMTP_FROM  与部署 .env 同一套凭据的副本
#   可选覆盖：WATCH_FAIL_THRESHOLD(3) WATCH_COOLDOWN_MIN(60) WATCH_DISK_CRIT_PCT(85)
#             WATCH_ASSISTANT_URL WATCH_PUBLIC_BRANDS_URL WATCH_REMOTE_HOST WATCH_REMOTE_LXC
# 发信失败 = 本单元失败（timer 的 OnFailure= 会再走一封），绝不静默跳过。
set -euo pipefail

ENV_FILE=${ENV_FILE:-/etc/forklift-watchdog.env}
STATE_FILE=${STATE_FILE:-/var/lib/forklift-watchdog/state}
FAIL_THRESHOLD=${WATCH_FAIL_THRESHOLD:-3}
COOLDOWN_S=$(( ${WATCH_COOLDOWN_MIN:-60} * 60 ))
DISK_CRIT_PCT=${WATCH_DISK_CRIT_PCT:-85}
ASSISTANT_URL=${WATCH_ASSISTANT_URL:-http://172.17.1.23:8000/assistant/api/health}
BRANDS_URL=${WATCH_PUBLIC_BRANDS_URL:-https://www.gccsmile.com/api/ai-assistant/diagnosis/brands}
REMOTE_HOST=${WATCH_REMOTE_HOST:-172.17.1.41}
REMOTE_LXC=${WATCH_REMOTE_LXC:-101}
EXPECT_CONTAINERS=(forklift-frontend-prod forklift-backend-prod forklift-pg-prod forklift-redis-prod forklift-libreoffice-prod)
TAG=forklift-watchdog

MODE=check
DRY_RUN=0
FAILED_UNIT=unknown
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  --alert-failure) MODE=alert-failure; FAILED_UNIT=${2:-unknown} ;;
esac

log() { echo "[$TAG] $*"; }

die_no_mail() { log "致命：$*"; exit 1; }

[ -r "$ENV_FILE" ] || die_no_mail "读不到 $ENV_FILE（发信与收件配置是硬前提）"
# shellcheck source=/dev/null
. "$ENV_FILE"
: "${WATCH_TO_EMAIL:?WATCH_TO_EMAIL 未设置}"
: "${SMTP_HOST:?SMTP_HOST 未设置}"

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

# ---- 探测面：每个 probe_* 返回 0=正常；非 0=故障，并把原因写进全局 REASON ----
REASON=""

probe_assistant_health() {
  local body
  body=$(curl -fsS -m 8 "$ASSISTANT_URL" 2>&1) || { REASON="助手内网 health 不可达：$body"; return 1; }
  grep -q '"ok":true' <<<"$body" || { REASON="助手 health 非 ok：$body"; return 1; }
}

probe_containers() {
  local ps line name state bad=()
  ps=$(docker ps -a --format '{{.Names}}|{{.Status}}' 2>/dev/null) || { REASON="docker ps 失败"; return 1; }
  for name in "${EXPECT_CONTAINERS[@]}"; do
    line=$(grep -m1 "^${name}|" <<<"$ps" || true)
    state=${line#*|}
    if [ -z "$line" ]; then bad+=("$name 不存在")
    elif [[ $state == unhealthy* ]]; then bad+=("$name unhealthy")
    elif [[ $state == Restarting* || $state == Exited* || $state == Created* || -z $state ]]; then bad+=("$name ${state% *}")
    fi
  done
  [ ${#bad[@]} -eq 0 ] || { REASON="容器异常：${bad[*]}"; return 1; }
}

probe_public_brands() {
  local body
  body=$(curl -fsS -m 12 "$BRANDS_URL" 2>&1) || { REASON="公网诊断端点不可达：$body"; return 1; }
  grep -qE '"code"[[:space:]]*:[[:space:]]*(0|200)' <<<"$body" || { REASON="公网诊断端点信封异常：${body:0:200}"; return 1; }
}

# 前端「在跑」≠「在服务」：frontend 是 network_mode: host，compose 里 healthcheck 被显式
# disable（#496 血账：host 网络下容器内 wget 跟随 301 → 证书不匹配 → 误报 unhealthy）。
# 判据沿用 deploy-remote.sh 的宿主侧探活（#483）：首选 https 443 /health，只认 200；
# 兜底 http 80 /health 同样只认 200 —— 80 端口的 server 级 return 301 抢在 location 前，
# 301 一律视为不通（2026-09-02 曾因此误杀一次 testing 冒烟）。
probe_frontend_health() {
  local code
  code=$(curl -sk -o /dev/null -w '%{http_code}' -m 8 "https://localhost:${WATCH_NGINX_HTTPS_PORT:-443}/health" 2>/dev/null || echo 000)
  if [ "$code" != "200" ]; then
    code=$(curl -s -o /dev/null -w '%{http_code}' -m 8 "http://localhost:${WATCH_NGINX_HTTP_PORT:-80}/health" 2>/dev/null || echo 000)
    [ "$code" = "200" ] || { REASON="前端 nginx 未在服务：https 与 http(301 不算) 的 /health 均非 200（本机回环，-k 跳证书）"; return 1; }
  fi
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
  out=$(ssh -o BatchMode=yes -o ConnectTimeout=8 "$REMOTE_HOST" \
          "pct exec $REMOTE_LXC -- df -P /" 2>&1) || { REASON="跨机取 LXC$REMOTE_LXC df 失败：${out:0:120}"; return 1; }
  pct=$(awk 'NR==2{print $5}' <<<"$out")
  [ -n "$pct" ] || { REASON="LXC$REMOTE_LXC df 输出无水位列"; return 1; }
  pct=$(disk_pct_of "$pct")
  [ "$pct" -lt "$DISK_CRIT_PCT" ] || { REASON="LXC$REMOTE_LXC 根盘已用 ${pct}%（阈值 ${DISK_CRIT_PCT}%）"; return 1; }
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
    [public_brands]=probe_public_brands
    [frontend_health]=probe_frontend_health
    [disk_host]=probe_disk_host
    [disk_lxc]=probe_disk_lxc
  )
  local order=(assistant_health containers public_brands frontend_health disk_host disk_lxc)
  local now=$(date +%s) down=() name fails alerted last reason
  for name in "${order[@]}"; do
    REASON=""
    if "${probe_fn[$name]}"; then
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
探测面：助手内网 health / pve-02 生产容器 / 公网诊断端点 / 前端 nginx /health / 本机与 LXC$REMOTE_LXC 磁盘水位"
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
  log "巡检完成：达告警级别 ${#down[@]}/${#order[@]}（阈值：同一面连续 ${FAIL_THRESHOLD} 次）${down[*]:+ ⇒ ${down[*]}}"
}

case "$MODE" in
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
