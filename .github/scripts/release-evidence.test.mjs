/**
 * ReleaseEvidence composite action 的测试（#2455 / #2920）
 *
 * 主 seam 是 action/workflow 的输入输出契约；gh CLI 作为外部边界使用 mock。
 * gateVerdict 只有在 SHA 对齐的 run success 且最新 deployment success 时为 green。
 */
import { test, describe, afterEach } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync, rmSync, readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ACTIONS_DIR = join(__dirname, "..", "actions", "release-evidence");
const WORKFLOWS_DIR = join(__dirname, "..", "workflows");
const SHA = "abc1234def5678";
const RUN_ID = "42424242";
const ENV = "testing";

let mockDir;

function setupMockGh(responses) {
  mockDir = mkdtempSync(join(tmpdir(), "gh-mock-"));

  // 模拟 authenticated gh CLI：按调用顺序返回原始 JSON / 文本与退出码。
  const mockGh = `#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const dir = __dirname;
const counterFile = path.join(dir, "counter");
const callNumber = Number(fs.existsSync(counterFile) ? fs.readFileSync(counterFile, "utf8") : 0) + 1;
fs.writeFileSync(counterFile, String(callNumber));
const responseFile = path.join(dir, "response-" + callNumber);
if (fs.existsSync(responseFile)) process.stdout.write(fs.readFileSync(responseFile, "utf8"));
const exitFile = path.join(dir, "exit-" + callNumber);
if (fs.existsSync(exitFile)) process.exit(Number(fs.readFileSync(exitFile, "utf8")));
`;
  writeFileSync(join(mockDir, "gh"), mockGh, { mode: 0o755 });

  responses.forEach((response, index) => {
    const callNumber = index + 1;
    if (response && typeof response === "object" && "exitCode" in response) {
      writeFileSync(join(mockDir, `response-${callNumber}`), response.stdout ?? "");
      writeFileSync(join(mockDir, `exit-${callNumber}`), String(response.exitCode));
      return;
    }
    writeFileSync(
      join(mockDir, `response-${callNumber}`),
      typeof response === "string" ? response : JSON.stringify(response),
    );
  });

  return mockDir;
}

// Git Bash uses /d/...; WSL already supplies /mnt/d/... paths.
function toBashPath(shellPath) {
  if (process.platform !== "win32") return shellPath;
  return shellPath.replace(/\\/g, "/").replace(/^([A-Za-z]):/, (_, drive) => `/${drive.toLowerCase()}`);
}

function runScript(scriptName, env = {}) {
  const outputFile = join(mockDir, "gh-output");
  const scriptPath = toBashPath(join(ACTIONS_DIR, scriptName));
  const mockDirBash = toBashPath(mockDir);
  const outputFileBash = toBashPath(outputFile);
  const command = `export PATH="${mockDirBash}:$PATH"; bash "${scriptPath}"`;

  try {
    const stdout = execFileSync("bash", ["-c", command], {
      encoding: "utf8",
      stdio: "pipe",
      env: {
        ...process.env,
        GITHUB_OUTPUT: outputFileBash,
        GITHUB_REPOSITORY: "owner/repo",
        ...env,
      },
    });
    return { exitCode: 0, output: readOutput(outputFile), stdout };
  } catch (error) {
    return {
      exitCode: error.status ?? 1,
      output: readOutput(outputFile),
      stdout: `${error.stdout ?? ""}${error.stderr ?? ""}`,
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

function outputValue(result, key) {
  return result.output.split("\n").find((line) => line.startsWith(`${key}=`))?.slice(key.length + 1);
}

function callCount() {
  try {
    return Number(readFileSync(join(mockDir, "counter"), "utf8").trim());
  } catch {
    return 0;
  }
}

function runResult(status, conclusion, headSha = SHA) {
  return { status, conclusion, headSha };
}

function deployment(id, createdAt, sha = SHA, environment = ENV) {
  return { id, sha, environment, created_at: createdAt };
}

function deploymentStatus(state, createdAt) {
  return { state, created_at: createdAt };
}

function assertVerdict(result, verdict) {
  assert.equal(outputValue(result, "verdict"), verdict, result.stdout || result.output);
}

afterEach(() => {
  if (mockDir) rmSync(mockDir, { recursive: true, force: true });
  mockDir = null;
});

// ── waitForRun ──────────────────────────────────────────────────────────

describe("waitForRun", () => {
  test("SHA 不匹配 → exit 1", () => {
    setupMockGh([runResult("completed", "success", "WRONG_SHA")]);
    const result = runScript("wait-for-run.sh", { INPUT_RUN_ID: RUN_ID, INPUT_SHA: SHA });
    assert.equal(result.exitCode, 1);
    assert.match(result.stdout, /SHA|sha/);
  });

  test("run completed/success → exit 0, output conclusion=success", () => {
    setupMockGh([runResult("completed", "success")]);
    const result = runScript("wait-for-run.sh", { INPUT_RUN_ID: RUN_ID, INPUT_SHA: SHA });
    assert.equal(result.exitCode, 0);
    assert.equal(outputValue(result, "conclusion"), "success");
    assert.equal(outputValue(result, "head_sha"), SHA);
  });

  test("run completed/failure → exit 1", () => {
    setupMockGh([runResult("completed", "failure")]);
    const result = runScript("wait-for-run.sh", { INPUT_RUN_ID: RUN_ID, INPUT_SHA: SHA });
    assert.equal(result.exitCode, 1);
    assert.match(result.stdout, /failure/);
  });

  test("in_progress → completed/success（轮询两次）", () => {
    setupMockGh([runResult("in_progress", null), runResult("completed", "success")]);
    const result = runScript("wait-for-run.sh", {
      INPUT_RUN_ID: RUN_ID,
      INPUT_SHA: SHA,
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 0);
    assert.equal(outputValue(result, "conclusion"), "success");
  });

  test("超时 → exit 1", () => {
    setupMockGh([runResult("in_progress", null)]);
    const result = runScript("wait-for-run.sh", {
      INPUT_RUN_ID: RUN_ID,
      INPUT_SHA: SHA,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assert.match(result.stdout, /timeout|超时|超过/);
  });
});

// ── checkDeployment ─────────────────────────────────────────────────────

describe("checkDeployment", () => {
  test("deployment success → exit 0", () => {
    setupMockGh([
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("check-deployment.sh", { INPUT_SHA: SHA, INPUT_ENVIRONMENT: ENV });
    assert.equal(result.exitCode, 0);
    assert.equal(outputValue(result, "deployment_id"), "1001");
    assert.equal(outputValue(result, "deployment_status"), "success");
  });

  for (const state of ["failure", "error", "cancelled"]) {
    test(`deployment ${state} → 立即 exit 1`, () => {
      setupMockGh([
        [deployment(1001, "2026-11-27T10:00:00Z")],
        [deploymentStatus(state, "2026-11-27T10:01:00Z")],
      ]);
      const result = runScript("check-deployment.sh", {
        INPUT_SHA: SHA,
        INPUT_ENVIRONMENT: ENV,
        INPUT_TIMEOUT: "5",
        INPUT_POLL_INTERVAL: "0",
      });
      assert.equal(result.exitCode, 1);
      assert.equal(outputValue(result, "deployment_status"), state);
      assert.equal(callCount(), 2);
    });
  }

  test("无记录 → 出现记录 success（轮询）", () => {
    setupMockGh([
      [],
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 0);
    assert.equal(outputValue(result, "deployment_status"), "success");
  });

  for (const state of ["pending", "in_progress"]) {
    test(`${state} → success（等待）`, () => {
      setupMockGh([
        [deployment(1001, "2026-11-27T10:00:00Z")],
        [deploymentStatus(state, "2026-11-27T10:01:00Z")],
        [deployment(1001, "2026-11-27T10:00:00Z")],
        [deploymentStatus("success", "2026-11-27T10:02:00Z")],
      ]);
      const result = runScript("check-deployment.sh", {
        INPUT_SHA: SHA,
        INPUT_ENVIRONMENT: ENV,
        INPUT_POLL_INTERVAL: "0",
      });
      assert.equal(result.exitCode, 0);
      assert.equal(outputValue(result, "deployment_status"), "success");
    });
  }

  test("TIMEOUT=0 时记录存在但 pending 不再视为绿", () => {
    setupMockGh([
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("pending", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assert.equal(outputValue(result, "deployment_status"), "pending");
  });

  test("状态查询失败 → fail closed", () => {
    setupMockGh([
      [deployment(1001, "2026-11-27T10:00:00Z")],
      { exitCode: 1, stdout: "" },
    ]);
    const result = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assert.equal(outputValue(result, "deployment_status"), "unknown");
  });

  test("超时无记录 → exit 1", () => {
    setupMockGh([[]]);
    const result = runScript("check-deployment.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
  });
});

// ── gateVerdict ─────────────────────────────────────────────────────────

describe("gateVerdict", () => {
  test("SHA 不匹配 → verdict=red, exit 1", () => {
    setupMockGh([runResult("completed", "success", "WRONG_SHA")]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
    assert.match(outputValue(result, "reason"), /SHA|sha/);
  });

  test("run success + 最新 deployment success → verdict=green", () => {
    setupMockGh([
      runResult("completed", "success"),
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
    });
    assert.equal(result.exitCode, 0);
    assertVerdict(result, "green");
    assert.equal(outputValue(result, "deployment_id"), "1001");
  });

  for (const conclusion of ["failure", "error", "cancelled"]) {
    test(`run ${conclusion} + deployment success → 立即 red`, () => {
      setupMockGh([
        runResult("completed", conclusion),
        [deployment(1001, "2026-11-27T10:00:00Z")],
        [deploymentStatus("success", "2026-11-27T10:01:00Z")],
      ]);
      const result = runScript("gate-verdict.sh", {
        INPUT_SHA: SHA,
        INPUT_ENVIRONMENT: ENV,
        INPUT_RUN_ID: RUN_ID,
        INPUT_TIMEOUT: "5",
        INPUT_POLL_INTERVAL: "0",
      });
      assert.equal(result.exitCode, 1);
      assertVerdict(result, "red");
      assert.equal(callCount(), 1);
    });
  }

  for (const state of ["failure", "error", "cancelled"]) {
    test(`run success + deployment ${state} → 立即 red`, () => {
      setupMockGh([
        runResult("completed", "success"),
        [deployment(1001, "2026-11-27T10:00:00Z")],
        [deploymentStatus(state, "2026-11-27T10:01:00Z")],
      ]);
      const result = runScript("gate-verdict.sh", {
        INPUT_SHA: SHA,
        INPUT_ENVIRONMENT: ENV,
        INPUT_RUN_ID: RUN_ID,
        INPUT_TIMEOUT: "5",
        INPUT_POLL_INTERVAL: "0",
      });
      assert.equal(result.exitCode, 1);
      assertVerdict(result, "red");
      assert.equal(callCount(), 3);
    });
  }

  test("最新 deployment success 覆盖旧 failure", () => {
    setupMockGh([
      runResult("completed", "success"),
      [
        deployment(1002, "2026-11-27T10:00:00Z"),
        deployment(9999, "2026-11-27T11:00:00Z", "other-sha"),
        deployment(1001, "2026-11-27T09:00:00Z"),
        deployment(9998, "2026-11-27T12:00:00Z", SHA, "production"),
      ],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
    });
    assert.equal(result.exitCode, 0);
    assertVerdict(result, "green");
    assert.equal(outputValue(result, "deployment_id"), "1002");
  });

  test("deployment 与 status 分页后仍取 created_at 最新", () => {
    setupMockGh([
      runResult("completed", "success"),
      [
        [deployment(1001, "2026-11-27T09:00:00Z")],
        [deployment(1002, "2026-11-27T10:00:00Z")],
      ],
      [
        [deploymentStatus("failure", "2026-11-27T10:01:00Z")],
        [deploymentStatus("success", "2026-11-27T10:02:00Z")],
      ],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 0);
    assertVerdict(result, "green");
    assert.equal(outputValue(result, "deployment_id"), "1002");
  });

  test("最新 deployment failure 覆盖旧 success", () => {
    setupMockGh([
      runResult("completed", "success"),
      [
        deployment(1001, "2026-11-27T09:00:00Z"),
        deployment(1002, "2026-11-27T10:00:00Z"),
      ],
      [deploymentStatus("failure", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
    assert.equal(outputValue(result, "deployment_id"), "1002");
  });

  test("同一 deployment 读最新状态：旧 failure 不覆盖新 success", () => {
    setupMockGh([
      runResult("completed", "success"),
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [
        deploymentStatus("success", "2026-11-27T10:03:00Z"),
        deploymentStatus("failure", "2026-11-27T10:01:00Z"),
      ],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
    });
    assert.equal(result.exitCode, 0);
    assertVerdict(result, "green");
  });

  test("同一 deployment 读最新状态：旧 success 不覆盖新 failure", () => {
    setupMockGh([
      runResult("completed", "success"),
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [
        deploymentStatus("failure", "2026-11-27T10:03:00Z"),
        deploymentStatus("success", "2026-11-27T10:01:00Z"),
      ],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
  });

  for (const state of ["pending", "in_progress"]) {
    test(`deployment ${state} → 等待到 success`, () => {
      setupMockGh([
        runResult("completed", "success"),
        [deployment(1001, "2026-11-27T10:00:00Z")],
        [deploymentStatus(state, "2026-11-27T10:01:00Z")],
        [deployment(1001, "2026-11-27T10:00:00Z")],
        [deploymentStatus("success", "2026-11-27T10:02:00Z")],
      ]);
      const result = runScript("gate-verdict.sh", {
        INPUT_SHA: SHA,
        INPUT_ENVIRONMENT: ENV,
        INPUT_RUN_ID: RUN_ID,
        INPUT_POLL_INTERVAL: "0",
      });
      assert.equal(result.exitCode, 0);
      assertVerdict(result, "green");
    });
  }

  test("run in_progress + deployment success → 等待 run success", () => {
    setupMockGh([
      runResult("in_progress", null),
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
      runResult("completed", "success"),
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 0);
    assertVerdict(result, "green");
  });

  test("超时无记录 → verdict=red", () => {
    setupMockGh([runResult("completed", "success"), []]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
  });

  test("run 查询失败 → fail closed", () => {
    setupMockGh([{ exitCode: 1, stdout: "" }]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
  });

  test("deployment 查询失败 → fail closed", () => {
    setupMockGh([runResult("completed", "success"), { exitCode: 1, stdout: "" }]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
  });

  test("deployment 状态查询失败 → fail closed", () => {
    setupMockGh([
      runResult("completed", "success"),
      [deployment(1001, "2026-11-27T10:00:00Z")],
      { exitCode: 1, stdout: "" },
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
  });

  test("deployment 查询返回损坏 JSON → fail closed", () => {
    setupMockGh([runResult("completed", "success"), "not-json"]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
    assert.equal(outputValue(result, "deployment_status"), "unknown");
  });

  test("瞬时 run/deployment/status 查询失败重试后仍可双成功", () => {
    setupMockGh([
      { exitCode: 1, stdout: "" },
      runResult("completed", "success"),
      { exitCode: 1, stdout: "" },
      [deployment(1001, "2026-11-27T10:00:00Z")],
      { exitCode: 1, stdout: "" },
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_RUN_ID: RUN_ID,
      INPUT_TIMEOUT: "5",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 0);
    assertVerdict(result, "green");
  });

  test("无 run-id 不能仅凭 deployment 放行", () => {
    setupMockGh([
      [deployment(1001, "2026-11-27T10:00:00Z")],
      [deploymentStatus("success", "2026-11-27T10:01:00Z")],
    ]);
    const result = runScript("gate-verdict.sh", {
      INPUT_SHA: SHA,
      INPUT_ENVIRONMENT: ENV,
      INPUT_TIMEOUT: "0",
      INPUT_POLL_INTERVAL: "0",
    });
    assert.equal(result.exitCode, 1);
    assertVerdict(result, "red");
    assert.match(outputValue(result, "reason"), /run-id|RUN_ID/);
    assert.equal(callCount(), 0);
  });
});

// ── composite action / workflow 接线契约 ─────────────────────────────────

describe("ReleaseEvidence workflow contract", () => {
  const action = readFileSync(join(ACTIONS_DIR, "action.yml"), "utf8");
  const testingSmoke = readFileSync(join(WORKFLOWS_DIR, "testing-smoke.yml"), "utf8");
  const cd = readFileSync(join(WORKFLOWS_DIR, "cd.yml"), "utf8");

  test("composite action 暴露 gateVerdict 的 verdict/reason，并接收 run-id", () => {
    assert.match(action, /id: gateVerdict/);
    assert.match(action, /run: bash "\$\{\{ github\.action_path \}\}\/gate-verdict\.sh"/);
    assert.match(action, /INPUT_RUN_ID: \$\{\{ inputs\.run-id \}\}/);
    assert.match(action, /value: \$\{\{ steps\.gateVerdict\.outputs\.verdict \}\}/);
    assert.match(action, /value: \$\{\{ steps\.gateVerdict\.outputs\.reason \}\}/);
  });

  test("testing-smoke 使用 composite gateVerdict，不自行解释 deployment", () => {
    assert.match(testingSmoke, /uses: \.\/\.github\/actions\/release-evidence/);
    assert.match(testingSmoke, /step: gateVerdict/);
    assert.match(testingSmoke, /run-id: \$\{\{ steps\.[^.]+\.outputs\.run[_-]id \}\}/);
    assert.match(testingSmoke, /environment: testing/);
    assert.doesNotMatch(testingSmoke, /release-evidence\/(?:wait-for-run|check-deployment|gate-verdict)\.sh/);
    assert.doesNotMatch(testingSmoke, /deployments\?per_page=100/);
    assert.doesNotMatch(testingSmoke, /INPUT_TIMEOUT="0"/);
  });

  test("production gate 传入 SHA 对齐的 run，并只消费 verdict", () => {
    assert.match(cd, /uses: \.\/\.github\/actions\/release-evidence/);
    assert.match(cd, /step: gateVerdict/);
    assert.match(cd, /run-id: \$\{\{ steps\.[^.]+\.outputs\.run[_-]id \}\}/);
    assert.match(cd, /environment: testing/);
    assert.doesNotMatch(cd, /release-evidence\/(?:wait-for-run|check-deployment|gate-verdict)\.sh/);
    assert.match(cd, /steps\.[^.]+\.outputs\.verdict/);
  });
});
