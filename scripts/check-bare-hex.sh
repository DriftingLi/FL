#!/usr/bin/env bash
# ===== 硬编码色值检查（深色模式防线）=====
#
# 背景：深色模式是靠翻 CSS 变量实现的，只对走 var() 的声明生效。
#       裸 hex 不参与变量链，暗色下保持亮色 → 「暗底亮块」崩坏。
#       而 var(--token, #fallback) 写法是安全的（token 生效时跟随主题）。
#
# 用法：
#   check-bare-hex.sh --all  [目录]    全量扫描，报告剩余进度（退出码恒为 0，不阻断）
#   check-bare-hex.sh --diff [base]   只查相对 base 的新增行（有违规则退出 1，CI 阻断用）
#                                     base 默认 origin/master
#
# 豁免规则（故意放过，不算违规）：
#   1. 同一行含 var(-- 的 —— 即 var(--token, #fallback) 防御写法
#   2. #fff / #ffffff / #000 / #000000 —— 中性色，深浅底都成立
#   3. 注释行（//、*、/*、<!--）—— 否则 <!-- #511：xxx --> 这类 Issue 编号会被误判
#   4. <script> 块 —— ECharts 等 canvas 色板属合理存在
#
# 只扫描 .vue 的 <template> 与 <style> 块。计数口径：按「行」计，一行多个色值算一处。
set -uo pipefail

MODE="${1:--all}"
ARG="${2:-}"
SCAN_DIR="${ARG:-frontend/src}"
DIFF_BASE="${ARG:-origin/master}"

# 单次 awk 完成「块提取 + 豁免判断」，避免逐行走 bash 循环（那样 146 个文件要跑 3 分钟）
# 兼容 mawk（Ubuntu 默认）：不使用 gawk 专有的 /i 修饰符与 \b，改用 tolower + 字符类
AWK_SCAN='
  /<style[^>]*>/    { ins  = 1; next }
  /<\/style>/       { ins  = 0; next }
  /<template[^>]*>/ { intp = 1; next }
  /<\/template>/    { intp = 0; next }
  !(ins || intp)    { next }
  # ── 多行注释块状态机（豁免续行，如 <!-- #554：... --> 的中间行）──
  /<!--/ && !/-->/ { inhtml = 1; next }
  /-->/            { inhtml = 0; next }
  inhtml           { next }
  /\/\*/ && !/\*\// { incss = 1; next }
  /\*\//           { incss = 0; next }
  incss            { next }
  # ── 单行豁免 ──
  /var\(--/         { next }
  /^[[:space:]]*(\/\/|\*|\/\*|<!--)/ { next }
  /^[[:space:]]*--[a-z0-9-]+[[:space:]]*:/ { next }
  {
    tmp = tolower($0)
    gsub(/#(fff|ffffff|000|000000)([^0-9a-f]|$)/, "", tmp)
    if (tmp ~ /#[0-9a-f]{3}/) printf "%s:%d:%s\n", FILENAME, FNR, $0
  }
'

scan_files() {
  # stdin: 文件清单（NUL 分隔）→ stdout: 违规行
  xargs -0 -r awk "$AWK_SCAN"
}

report_all() {
  local tmp
  tmp="$(mktemp)"
  find "$SCAN_DIR" -name '*.vue' -type f -print0 2>/dev/null | sort -z | scan_files >"$tmp"

  echo "===== 裸 hex 全量扫描（排除 var() fallback / #fff / #000 / 注释 / script 块）====="
  echo "扫描目录: $SCAN_DIR"
  echo "---"
  if [[ -s "$tmp" ]]; then
    cat "$tmp"
    echo "---"
    echo "文件数: $(cut -d: -f1 "$tmp" | sort -u | wc -l | tr -d ' ')"
    echo "裸 hex 处数: $(wc -l <"$tmp" | tr -d ' ')  (按行计)"
  else
    echo "无违规。全站样式已 100% 走 token / 原子类。"
  fi
  rm -f "$tmp"
  return 0
}

report_diff() {
  if ! git rev-parse --verify "$DIFF_BASE" >/dev/null 2>&1; then
    echo "[check-bare-hex] 无法解析 base ref: $DIFF_BASE（CI 上请先 git fetch）" >&2
    return 1
  fi

  local added tmp
  added="$(mktemp)"
  tmp="$(mktemp)"

  # 收集新增行的 "file:lineno"，供后续与全量扫描结果求交集
  git diff -U0 "$DIFF_BASE"...HEAD -- '*.vue' | awk '
    /^\+\+\+ b\// { f = substr($0, 7); next }
    /^@@/         { match($0, /\+[0-9]+/); ln = substr($0, RSTART + 1, RLENGTH - 1) + 0; next }
    /^\+/ && !/^\+\+\+/ { printf "%s:%d\n", f, ln; ln++ }
  ' | sort -u >"$added"

  if [[ ! -s "$added" ]]; then
    echo "[check-bare-hex] 相对 $DIFF_BASE 无 .vue 新增行，跳过。"
    rm -f "$added" "$tmp"
    return 0
  fi

  # 只扫改动涉及的 .vue 文件，再筛出落在新增行上的那些
  cut -d: -f1 "$added" | sort -u | tr '\n' '\0' | scan_files >"$tmp.raw"
  awk -v FS=: 'NR==FNR { a[$1 ":" $2] = 1; next } { k = $1 ":" $2; if (k in a) print }' \
    "$added" "$tmp.raw" >"$tmp"

  if [[ -s "$tmp" ]]; then
    echo "===== 新增行含硬编码色值（深色模式下不会跟随主题）=====" >&2
    cat "$tmp" >&2
    echo "---" >&2
    echo "共 $(wc -l <"$tmp" | tr -d ' ') 处。" >&2
    echo "请改用 design-tokens.css 的 CSS 变量或 Tailwind 原子类；" >&2
    echo "若确需保留（如 canvas 色板、var() fallback），请加注释说明。" >&2
    rm -f "$added" "$tmp" "$tmp.raw"
    return 1
  fi

  echo "[check-bare-hex] 新增行无硬编码色值，通过。"
  rm -f "$added" "$tmp" "$tmp.raw"
  return 0
}

case "$MODE" in
--all) report_all ;;
--diff) report_diff ;;
*)
  echo "用法: $0 --all [目录] | --diff [base]" >&2
  exit 2
  ;;
esac
