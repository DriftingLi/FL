#!/usr/bin/env bash
# CI 环境层判据：git 的路径输出不得被 C 风格转义（#2885，补 #2873 未闭环的 CI 侧）。
#
# 为什么需要：本仓工作区路径含中文目录（training-app/叉车维修培训学员端跨端应用/）。
# git 默认 core.quotepath=true，会把非 ASCII 路径写成
#   "training-app/\346\234\211\350\275\246..."
# 于是任何「取路径 -> 判前缀 / 后缀 / 相等」的脚本静默失配（#2873 已在本地侧用
# scripts/install-hooks.ps1 钉住；CI 不装钩子，故由各 workflow 顶层 env 的
# GIT_CONFIG_COUNT / GIT_CONFIG_KEY_0 / GIT_CONFIG_VALUE_0 三件套钉住）。
#
# 判据（fail-closed，任一不成立即红）：
#   ① git config --get core.quotepath 读数是 false；
#   ② git ls-files 对**真实中文路径**输出字面量（grep 命中）——不绑定某一次 PR 的 diff，
#      每个跑测试的 job 都能无条件证一次；
#   ③ 反向对照：显式 -c core.quotepath=true 时同一条 grep 必须不中
#      （证明 ② 不是恒绿的空判据）。
#
# 用法：bash scripts/check-git-quotepath.sh（任何 cwd 都行，脚本自己跳到仓库根）
set -euo pipefail

fail() {
    echo "::error::$1" >&2
    echo "GIT_QUOTEPATH_RESULT verdict=fail reason=$2" >&2
    exit 1
}

root=$(git rev-parse --show-toplevel 2>/dev/null) || fail "不在 git 工作区内，判据无从取证" "not-a-git-worktree"
cd "$root"

version=$(git --version 2>/dev/null || echo 'git <unknown>')
quotepath=$(git config --get core.quotepath || true)
echo "GIT_QUOTEPATH git=${version#git version } core.quotepath=${quotepath:-<unset>} GIT_CONFIG_COUNT=${GIT_CONFIG_COUNT:-<unset>}"

[ "$quotepath" = "false" ] || fail "core.quotepath=${quotepath:-<unset>}，期望 false：检查 workflow 顶层 env 的 GIT_CONFIG_* 三件套是否还在" "quotepath-not-false"

needle='training-app/叉车维修培训学员端跨端应用'
literal_hits=$(git ls-files | grep -cF -- "$needle" || true)
[ "$literal_hits" -gt 0 ] || fail "真实中文路径没有被原样输出（hits=0）：路径仍被转义" "literal-miss"

quoted_hits=$(git -c core.quotepath=true ls-files | grep -cF -- "$needle" || true)
[ "$quoted_hits" -eq 0 ] || fail "反向对照失效：quotepath=true 时仍命中 $quoted_hits 行，判据没有牙" "negative-control-hit"

echo "GIT_QUOTEPATH_RESULT verdict=ok core.quotepath=false literal_hits=$literal_hits quoted_hits=0 git=${version#git version }"
