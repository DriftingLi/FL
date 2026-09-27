# Upstream issue report — `automator.launch()` fails on Windows with Node >= 18.20.2 (EINVAL), and the error blames `cliPath`

**Status of this file:** drafted, **NOT filed**. See "Why it is not filed" at the bottom.

- **Package:** `miniprogram-automator@0.12.1` (latest on npm as of 2026-09-18)
- **npm `repository` field:** `git@git.code.oa.com:devtools/automator.git`
- **npm `bugs` field:** absent (no public issue tracker declared)
- **Environment reproduced on:** Windows, Node v24.14.0, WeChat DevTools Stable 2.01.2510290
- **Upstream source:** `out/Launcher.js` — a single-line minified bundle (1 line, 3370 bytes)

---

## Summary

`automator.launch()` can never succeed on Windows with Node >= 18.20.2, because it calls `child_process.spawn()` on a `.bat`/`.cmd` path without `shell: true`. Node's CVE-2024-27980 hardening made that a synchronous `EINVAL` throw.

Worse than the failure is the diagnosis it produces: the thrown message is
`Failed to launch wechat web devTools, please make sure cliPath is correctly specified`,
which points the user at `cliPath` — the one thing that is actually correct.

## Root cause (exact source)

In `out/Launcher.js`, at byte offsets into the single-line bundle:

| offset | code |
| --- | --- |
| 2013 | `const t=child_process_1.default.spawn(e,n,_)` — `e` = `cliPath`, `n` = args array, `_ = { stdio: "ignore" }` (plus `cwd` when given) |
| 2058 | `t.on("error",(t=>{p=t}))` |
| 2134 | `catch(t){p=t}` |
| 2351/2357 | `throw Error("Failed to launch wechat web devTools, please make sure cliPath is correctly specified")` |

The string `shell` occurs **0 times in the entire package**.

On Windows, `cliPath` is normally `cli.bat`. Since Node 18.20.2 / 20.12.2 / 21.7.3, passing a `.bat` or `.cmd` file to `child_process.spawn()` **without** the `shell` option errors with `EINVAL` (Node's own advisory for CVE-2024-27980 states this verbatim; the sanctioned fix is `{ shell: true }`).

Two consequences, both observable:

1. **`spawn()` throws synchronously**, so the `catch (t) { p = t }` block is what sets the error — the `.on("error")` handler never fires. A caller that guards with only an `error` listener would crash outright rather than get the friendly message.
2. Because `p` is set, `waitUntil(...)` returns without ever attempting `connectTool(...)`, and the misleading `cliPath` message is thrown.

## Reproduction

Measured on Windows + Node v24.14.0.

**(a) Minimal — a harmless `.bat`:**

```js
const cp = require('child_process');
cp.spawn('some.bat', ['auto', '--project', 'D:\\proj\\dist'], { stdio: 'ignore' });
// throws synchronously: EINVAL
```

Also reproduces with `spawn('npm.cmd', ...)` → `EINVAL`. `spawn('node', ...)` is unaffected — it is specific to `.bat`/`.cmd`.

**(b) Through the library itself:**

```js
const automator = require('miniprogram-automator');
await automator.launch({ cliPath: '<real cli.bat>', projectPath: '<real dist>' });
// fails after ~13 ms with:
//   Failed to launch wechat web devTools, please make sure cliPath is correctly specified
```

The `.bat` never executes.

**(c) A/B proof that `cliPath` and the args were always correct:**

- Unpatched: fails in ~80 ms, `.bat` never runs.
- With `shell: true` injected into that one `spawn()` call: the `.bat` **runs** and receives
  `auto --project <proj> --auto-port 19423`.

So the executable path and the argument list were right all along; only the spawn mode was wrong.

## Requested fix

**1. Required — make the spawn Windows-aware.** Add `shell: true` when the resolved `cliPath` ends in `.bat`/`.cmd` (rather than unconditionally, so `.exe`/POSIX paths keep the current, safer behaviour).

**2. Required companion — quote arguments that need it.** `shell: true` concatenates arguments without escaping; Node emits `DEP0190` for exactly this
(`DeprecationWarning: Passing args to a child process with shell option true can lead to security vulnerabilities, as the arguments are not escaped, only concatenated.`).

This matters here because real-world values contain spaces: `--project`, `--auto-account`, `--ticket` and `cliPath` itself (e.g. a devtools install under `C:\Program Files\...`). Where the arguments go after that is outside our control — `cli.bat` forwards them to a Node CLI, and we could not trace that hop (see "Not verified"). So please quote `--project` / `--auto-account` / `--ticket` values (or invoke `cmd.exe /c` with pre-quoted arguments, or invoke the underlying `.js` entry point directly instead of the `.bat` shim — the last option avoids the shell entirely and would be the most robust).

**3. Recommended — stop misattributing the error.** Distinguish "the process could not be spawned at all" from "the CLI ran but the automation port never came up". Surfacing `err.code` (`EINVAL` vs `ENOENT`) in the thrown message would have saved a long detour: with `EINVAL` the path is fine, and the message "please make sure cliPath is correctly specified" is actively wrong.

## Workaround for users (no library change needed)

Start the automation port externally, then connect — this is the route that works today:

```
cli.bat auto --project <dist> --auto-port <port>     # externally, NOT via launch()
# wait until the port answers Tool.getInfo with SDKVersion, and App.getPageStack responds
automator.connect({ wsEndpoint: 'ws://127.0.0.1:<port>' })
```

For reference, in a fully ready session the three milestones are far apart
(port accepts ≈1 s → `Tool.getInfo` returns `SDKVersion` ≈1–3 s → `App.getPageStack` starts answering ≈24–34 s), so connecting too early produces an unrelated `Failed connecting…` message.

## Not verified (stated so the report is not overclaiming)

- Whether an **unquoted path containing a space** actually truncates in practice. A `%*` echo in a probe `.bat` shows `cmd.exe` reassembles the raw line, which looks fine; the risk is real for the next hop (`cli.bat` → Node CLI argument parsing). We flag the quoting requirement as a precaution grounded in `DEP0190`, not as a measured failure.
- Whether downstream consumers (`uni-automator` and similar) reach this code path. `@dcloudio/uni-automator@2.0.0` contains **0 references** to `miniprogram-automator`, so the widely repeated claim that it "goes through `Automator.launch`" appears to be false. (Separately, `uni-automator` has its own same-class exposure — its compile step spawns `npm.cmd` without `shell`, which also throws `EINVAL` — but that is a different project.)
- Whether newer releases already fixed this. `0.12.1` is the latest on npm; the version predates the Node hardening.

## Disclosure

This report was produced with AI assistance during triage of an unrelated downstream repository. The manual reproduction, source inspection and A/B patch described above were executed on the reporter's machine.
