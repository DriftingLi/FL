# #2729 hx-run 基线判据实跑证据

## 修复前（#2728 收口时两次真实部署）

```
HX_RUN_DEPLOY deployed=false www=none->none pid_before=none pid_after=none foreground= reason=设备侧事实与基线完全一致（资源 mtime 未前进、进程 pid 未变；前台=）
```

设备侧事实同时是：`apps/__UNI__1C1D180/www` 的 mtime 从 09:54 前进到 10:24，冷启动行为随代码改变。
即：读空被写成了「与基线完全一致」，且 reason 里没有任何「没测到」的痕迹。

## 修复后（本票提交 5be3c228，`-Force` 真运行）

```
>>> [deploy] 基线：/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1795227856->1795227856
HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1795227856->1795240045 pid_before=io.dcloud.uniappx=19798 pid_after=io.dcloud.uniappx=30398 baseline=ok after=ok foreground=io.dcloud.uniappx reason=资源已落到设备：/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www 的 mtime 相对基线前进
HX_RUN mode=incremental compile=240 deploy=0 total=293 exit=ok
```

## 改动

- 基线/现值读取各重试一次、单次预算 20 秒；仍读空则在 `Failures` 里记 `www:<路径>:unreadable` / `pid:ps:unreadable`。
- 机检行新增 `baseline=<ok|unreadable>`、`after=<ok|unreadable>`；基线读空时 reason 明说「基线未取到 ⇒ 无从判断」，
  不再复用「完全一致」。
- 把留在 `try/finally` 之后（不可达）的 `$script:TargetSerial` 初始化移到首次设备取数之前。

## 契约

`utils/hxRunContract.test.js`、`utils/buildDeployContract.test.js`：24 条通过（含 C11 基线相对判据、C14 观察窗）。
