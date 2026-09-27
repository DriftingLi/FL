## 仓内收口（COMPLETED 指仓内动作，不是上游缺陷已修）

### 已完成

| 动作 | 产物 |
| --- | --- |
| 实证本票主张 | 自行复现：`spawn` 一个 `.bat`/`.cmd` 在本机 Node 上**同步抛 `EINVAL`**（3ms）；A/B 对照证明打上 `shell: true` 后 `.bat` 真的执行 ⇒ **`cliPath`/args 一直是对的，报错文案确实在错误归因** |
| 写回 ADR | **PR #1167**（squash `1a03535c`）—— 修正移动端 `docs/adr/0008-移动端验收门与证据.md` 第 6 条对 `uni-automator` 的错误归因 |
| ② 门不受影响 | 已核实：门脚本零个 `.launch(` 调用，探针走 `automator.connect({ wsEndpoint })` |

### 收口时的两处订正（照实记）

1. **原判断「内容躺在未合并的 worktree 里、本票只需等合并」是错的。** 实际 `origin/master` 本来就有这条（由已合并的 **#1136** 写入）。误导来源：`D:\FL` 的工作副本落在分叉的本地 `master` 上，工作树里那份 ADR 是旧版本，对它 grep 会得到假阴性。
2. **票面「`uni-automator` 的 mp-weixin 路径内部用 `Automator.launch`」被实测推翻**：`@dcloudio/uni-automator@2.0.0` 的依赖表里没有 `miniprogram-automator`，全包对它与 `Automator.launch` 的引用**均为 0 处**。**同类缺陷确实存在，但在另一条路径上**：其编译步骤在 Windows 下取 `npm.cmd` 去 spawn 且同样没有 `shell`（全包 `shell` 出现 0 次），触发条件是 `package.json` 里命中 `<platform>:<mode>` 脚本条目**才**会抛 `EINVAL` —— 不是「只要用 uni-automator 就必撞」。此结论已写进上述 PR。

### 仍然未解决的（因此本票关闭 ≠ 缺陷消失）

- **上游缺陷本身未修**：`miniprogram-automator@0.12.1` 的 `automator.launch()` 在 Windows + Node ≥ 18.20.2 上依然必然失败。
- **上游 issue 未提交**：该包 npm `repository` 指向 `git@git.code.oa.com:devtools/automator.git`（腾讯内网 GitLab），npm 无 `bugs` 字段，公网无镜像，本机无内网凭据 ⇒ **提交卡在权限与可达性，不是能力问题**。报告草稿已备好（含精确源码偏移、复现命令、A/B 证据，以及「只加 `shell: true` 不够：Node 会发 DEP0190，args 未转义/仅拼接，须同时给 `--project` 等值加引号」）。

### 现行可用绕法（② 门即此形态）

外部启动自动化端口再连接，**不要**用 `launch()`：

```
cli.bat auto --project <dist> --auto-port <port>
# 等端口应答 Tool.getInfo（带 SDKVersion）且 App.getPageStack 开始应答
automator.connect({ wsEndpoint: 'ws://127.0.0.1:<port>' })
```

就绪窗口很宽（端口接受 ≈1s → `SDKVersion` ≈1–3s → `getPageStack` ≈24–34s），连太早会得到无关的 `Failed connecting…` 文案。

按与 **#1137** 一致的口径关闭本票（两者的仓内交付物均已完成、外部依赖均在票外），并摘除已无意义的 `ready-for-mobile-agent`。
