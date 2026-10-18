/**
 * 逐套件心跳 reporter（#1702）—— 挂在 `jest.config.behavior.js` 的 `reporters` 上。
 *
 * 要解决的问题不是「日志不好看」，是**判读**：行为守护那腿串行跑（`-i`，#1410 不动），
 * 单套件最长可达数分钟 ⇒ 终端与日志里长时间**什么都没有**。「静默」于是被读成「卡死」，
 * 三次误判都出在这里（#1410 / #1656 盘点 / 2026-10 那次；根因账见 #1702）。
 * 本 reporter 把「哪个套件刚跑完、第几个、用了多久、累计多久」写成**一行一个事实**：
 * 静默从「整轮」压缩到「单个套件之内」，而卡死的判据从「没输出」变成「同一行之后不再有新行」。
 *
 * 它**不**改任何判据：不设超时、不重试、不吞失败、不替代默认 reporter（两个 reporter 并存，
 * `reporters: ['default', ...]`）—— 默认 reporter 的汇总行仍是 `Write-UnitGateArtifacts` 取数的唯一来源。
 *
 * 行格式（供人读 + 供 grep，均为 `key=value` 空格分隔的单行）：
 *   JEST_SUITE_PLAN total=<N> started=<hh:mm:ss> note=...
 *   JEST_SUITE i=<k>/<N> status=PASS|FAIL secs=<s> elapsed=<s> tests=<n> failed=<n> file=utils/x.test.js
 *   JEST_SUITE_DONE total=<N> passed=<n> failed=<n> elapsed=<s>
 */
'use strict';

const path = require('node:path');

/** 本机时钟 hh:mm:ss —— 日志里「这一跑什么时候开始的」只能靠它，机检行里没有这一枚。 */
function clock() {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, '0');
  return pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
}

function say(line) {
  process.stdout.write(line + '\n');
}

class SuiteProgressReporter {
  constructor() {
    this.startedAt = 0;
    this.total = 0;
    this.done = 0;
    this.passed = 0;
    this.failed = 0;
  }

  onRunStart(results) {
    this.startedAt = Date.now();
    this.done = 0;
    this.passed = 0;
    this.failed = 0;
    this.total = (results && results.numTotalTestSuites) || 0;
    say(
      'JEST_SUITE_PLAN total=' + this.total +
        ' started=' + clock() +
        ' note=逐套件心跳行将逐个出现；整轮静默只会发生在单个超长套件内（设计内，不是卡死）'
    );
  }

  onTestResult(test, result) {
    this.done += 1;
    const execError = !!(result && result.testExecError);
    const failedTests = (result && result.numFailingTests) || 0;
    const passedTests = (result && result.numPassingTests) || 0;
    const ok = !execError && failedTests === 0;
    if (ok) {
      this.passed += 1;
    } else {
      this.failed += 1;
    }
    const rt = result && result.perfStats && typeof result.perfStats.runtime === 'number' ? result.perfStats.runtime : null;
    const secs = rt === null ? '?' : (rt / 1000).toFixed(1);
    const elapsed = ((Date.now() - this.startedAt) / 1000).toFixed(1);
    const abs = test && test.path ? test.path : '';
    const rel = abs ? path.relative(process.cwd(), abs).split(path.sep).join('/') : '?';
    say(
      'JEST_SUITE i=' + this.done + '/' + (this.total || '?') +
        ' status=' + (ok ? 'PASS' : 'FAIL') +
        ' secs=' + secs +
        ' elapsed=' + elapsed +
        ' tests=' + (passedTests + failedTests) +
        ' failed=' + failedTests +
        ' file=' + rel
    );
  }

  onRunComplete(contexts, results) {
    const elapsed = ((Date.now() - this.startedAt) / 1000).toFixed(1);
    const total = (results && results.numTotalTestSuites) || 0;
    const passed = (results && results.numPassedTestSuites) || 0;
    const failed = (results && results.numFailedTestSuites) || 0;
    say('JEST_SUITE_DONE total=' + total + ' passed=' + passed + ' failed=' + failed + ' elapsed=' + elapsed);
  }
}

module.exports = SuiteProgressReporter;
