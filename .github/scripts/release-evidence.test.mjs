/**
 * ReleaseEvidence composite action 的测试（#2455）
 *
 * 测试 shell 脚本的逻辑：waitForRun / checkDeployment / gateVerdict
 * 通过 mock `gh` CLI 返回预设响应，验证轮询、SHA 校验、超时、错误处理。
 */
import { test, describe, beforeEach, afterEach } from "node:test";
import assert from "node:assert";
import { execSync } from "node:child_process";
import { mkdtempSync, writeFileSync, rmSync, readFileSync, mkdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ACTIONS_DIR = join(__dirname, "..", "actions", "release-evidence");

// ── mock gh helpers ──────────────────────────────────────────────────────

let mockDir;

function setupMockGh(responses) {
  mockDir = mkdtempSync(join(tmpdir(), "gh-mock-"));

  // mock gh: reads counter, returns response-N
  const mockGh = `#!/bin/bash
DIR="$(dirname "$0")"
COUNTER_FILE="$DIR/counter"
CALL_NUM=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
CALL_NUM=$((CALL_NUM + 1))
echo "$CALL_NUM" > "$COUNTER_FILE"
RESPONSE_FILE="$DIR/response-$CALL_NUM"
if [ -f "$RESPONSE_FILE" ]; then cat "$RESPONSE_FILE"; fi
`;
  writeFileSync(join(mockDir, "gh"), mockGh, { mode: 0o755 });

  responses.forEach((resp, i) => {
    writeFileSync(join(mockDir, `response-${i + 1}`), typeof resp === "string" ? resp : JSON.stringify(resp));
  });

  return mockDir;
}

// WSL bash needs /mnt/d/... paths instead of D:/...
function toWslPath(winPath) {
  return winPath.replace(/\\/g, "/").replace(/^([A-Za-z]):/, (_, d) => `/mnt/${d.toLowerCase()}`);
}

function runScript(scriptName, env = {}) {
  const outputFile = join(mockDir, "gh-output");
  const scriptPath = toWslPath(join(ACTIONS_DIR, scriptName));
  const mockDirWsl = toWslPath(mockDir);
  const outputFileWsl = toWslPath(outputFile);

  // Build env assignments inside bash -c so WSL inherits them correctly
  const allEnv = {
    GITHUB_OUTPUT: outputFileWsl,
    GITHUB_REPOSITORY: "owner/repo",
    ...env,
  };
  const envExports = Object.entries(allEnv)
    .map(([k, v]) => `export ${k}="${String(v).replace(/"/g, '\\"')}"`)
    .join("; ");

  try {
    execSync(`bash -c '${envExports}; export PATH="${mockDirWsl}:$PATH"; bash "${scriptPath}"'`, {
      encoding: "utf8",
      stdio: "pipe",
    });
    return { exitCode: 0, output: readOutput(outputFile), stdout: "" };
  } catch (err) {
    return {
      exitCode: err.status ?? 1,
      output: readOutput(outputFile),
      stdout: (err.stdout ?? "") + (err.stderr ?? ""),
    };
  }
}

function readOutput(file) {
  try {
    return readFileSync(file, "utf8").trim();
  } catch {
    return "";
  }
}

afterEach(() => {
  if (mockDir) rmSync(mockDir, { recursive: true, force: true });
  mockDir = null;
});

// ── waitForRun ──────────────────────────────────────────────────────────

describe("waitForRun", () => {
  const SHA = "abc1234def5678";
  const RUN_ID = "42424242";

  test("SHA 不匹配 → exit 1", () => {
    setupMockGh([
      // gh run view returns headSha that doesn't match
      "completed/success/WRONG_SHA",
    ]);
    const r = runScript("wait-for-run.sh", {
      INPUT_RUN_ID: RUN_ID,
      INPUT_SHA: SHA,
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.stdout.includes("SHA") || r.stdout.includes("sha"), `stderr should mention SHA mismatch, got: ${r.stdout}`);
  });

  test("run completed/success → exit 0, output conclusion=success", () => {
    setupMockGh([
      `completed/success/${SHA}`,
    ]);
    const r = runScript("wait-for-run.sh", {
      INPUT_RUN_ID: RUN_ID,
      INPUT_SHA: SHA,
    });
    assert.strictEqual(r.exitCode, 0);
    assert.ok(r.output.includes("conclusion=success"), `output should have conclusion=success, got: ${r.output}`);
    assert.ok(r.output.includes(`head_sha=${SHA}`), `output should have head_sha, got: ${r.output}`);
  });

  test("run completed/failure → exit 1", () => {
    setupMockGh([
      `completed/failure/${SHA}`,
    ]);
    const r = runScript("wait-for-run.sh", {
      INPUT_RUN_ID: RUN_ID,
      INPUT_SHA: SHA,
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.stdout.includes("failure"), `stderr should mention failure, got: ${r.stdout}`);
  });

  test("in_progress → completed/success（轮询两次）", () => {
    setupMockGh([
      `in_progress/-/${SHA}`,
      `completed/success/${SHA}`,
    ]);
    const r = runScript("wait-for-run.sh", {
      INPUT_RUN_ID: RUN_ID,
      INPUT_SHA: SHA,
      INPUT_POLL_INTERVAL: "1",
    });
    assert.strictEqual(r.exitCode, 0);
    assert.ok(r.output.includes("conclusion=success"));
  });

  test("超时 → exit 1", () => {
    setupMockGh([
      `in_progress/-/${SHA}`,
      `in_progress/-/${SHA}`,
      `in_progress/-/${SHA}`,
    ]);
    const r = runScript("wait-for-run.sh", {
      INPUT_RUN_ID: RUN_ID,
      INPUT_SHA: SHA,
      INPUT_TIMEOUT: "1",
      INPUT_POLL_INTERVAL: "1",
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.stdout.includes("超过") || r.stdout.includes("超时") || r.stdout.includes("timeout") || r.stdout.includes("Timeout"), `should mention timeout, got: ${r.stdout}`);
  });
});

// ── checkDeployment ─────────────────────────────────────────────────────

describe("checkDeployment", () => {
  const SHA = "abc1234def5678";
  const ENV = "testing";

  test("deployment success → exit 0", () => {
    setupMockGh([
      // deployments query → DEP_ID
      "1001",
      // statuses query → STATE
      "success",
    ]);
    const r = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
    });
    assert.strictEqual(r.exitCode, 0);
    assert.ok(r.output.includes("deployment_id=1001"));
    assert.ok(r.output.includes("deployment_status=success"));
  });

  test("deployment failure → exit 1", () => {
    setupMockGh([
      "1001",
      "failure",
    ]);
    const r = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.stdout.includes("failure"), `should mention failure, got: ${r.stdout}`);
  });

  test("无记录 → 出现记录 success（轮询）", () => {
    setupMockGh([
      "",     // no deployment yet
      "1001", // now exists
      "success",
    ]);
    const r = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_POLL_INTERVAL: "1",
    });
    assert.strictEqual(r.exitCode, 0);
    assert.ok(r.output.includes("deployment_status=success"));
  });

  test("超时无记录 → exit 1", () => {
    setupMockGh([
      "",
      "",
      "",
    ]);
    const r = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_TIMEOUT: "1",
      INPUT_POLL_INTERVAL: "1",
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.stdout.includes("等不到") || r.stdout.includes("超过") || r.stdout.includes("超时") || r.stdout.includes("timeout") || r.stdout.includes("Timeout"));
  });
});

// ── gateVerdict ─────────────────────────────────────────────────────────

describe("gateVerdict", () => {
  const SHA = "abc1234def5678";
  const ENV = "testing";

  test("SHA 不匹配 → verdict=red, exit 1", () => {
    setupMockGh([
      "WRONG_SHA", // run headSha
    ]);
    const r = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: "42424242",
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.output.includes("verdict=red"));
    assert.ok(r.output.includes("SHA") || r.output.includes("sha"), `reason should mention SHA, got: ${r.output}`);
  });

  test("deployment success → verdict=green, exit 0", () => {
    setupMockGh([
      SHA,     // run headSha matches
      "1001",  // DEP_ID
      "success", // STATE
    ]);
    const r = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: "42424242",
    });
    assert.strictEqual(r.exitCode, 0);
    assert.ok(r.output.includes("verdict=green"));
  });

  test("deployment failure → verdict=red, exit 1", () => {
    setupMockGh([
      SHA,
      "1001",
      "failure",
    ]);
    const r = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: "42424242",
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.output.includes("verdict=red"));
    assert.ok(r.output.includes("failure"));
  });

  test("超时 → verdict=red, exit 1", () => {
    setupMockGh([
      SHA,
      "",     // no deployment
      "",
      "",
    ]);
    const r = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: "42424242",
      INPUT_TIMEOUT: "1",
      INPUT_POLL_INTERVAL: "1",
    });
    assert.strictEqual(r.exitCode, 1);
    assert.ok(r.output.includes("verdict=red"));
  });

  test("无 run-id → 跳过 SHA 校验，直接查 deployment", () => {
    setupMockGh([
      "1001",
      "success",
    ]);
    const r = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
    });
    assert.strictEqual(r.exitCode, 0);
    assert.ok(r.output.includes("verdict=green"));
  });
});
