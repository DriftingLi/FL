#!/usr/bin/env node
// ReleaseEvidence 的唯一判定核心：查询、状态分类与轮询都在这里。
// waitForRun / checkDeployment / gateVerdict shell 只是同一模块的薄 adapter。
import { appendFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const MODE = process.argv[2];
const SHA = process.env.INPUT_SHA ?? "";
const RUN_ID = process.env.INPUT_RUN_ID ?? "";
const ENVIRONMENT = process.env.INPUT_ENVIRONMENT || "testing";
const REPOSITORY = process.env.GITHUB_REPOSITORY ?? "";
const OUTPUT_FILE = process.env.GITHUB_OUTPUT ?? "";
const TIMEOUT_TEXT = process.env.INPUT_TIMEOUT || (MODE === "waitForRun" ? "1800" : "480");
const POLL_TEXT = process.env.INPUT_POLL_INTERVAL || "30";
const RED_STATES = new Set(["failure", "error", "cancelled"]);
const WAIT_RUN_STATES = new Set(["queued", "requested", "waiting", "pending", "in_progress"]);
const WAIT_DEPLOYMENT_STATES = new Set(["pending", "in_progress", "queued", "waiting", "none"]);

function emit(key, value) {
  const safeValue = String(value ?? "").replace(/[\r\n]+/g, " ");
  appendFileSync(OUTPUT_FILE, `${key}=${safeValue}\n`);
}

function emitDeployment(deployment) {
  emit("deployment_id", deployment?.id ?? "");
  emit("deployment_status", deployment?.state ?? "unknown");
}

function red(reason, deployment) {
  if (deployment !== undefined) emitDeployment(deployment);
  emit("verdict", "red");
  emit("reason", reason);
  console.error(`::error::${reason}`);
  return { state: "red", reason };
}

function green(reason, deployment) {
  emitDeployment(deployment);
  emit("verdict", "green");
  emit("reason", reason);
  console.log(`ReleaseEvidence green: ${reason}`);
  return { state: "success", reason };
}

function gh(args) {
  // 通过 bash 传 argv，兼容 Git Bash/WSL 下无扩展名的 mock gh，也避免 shell 拼接输入。
  return execFileSync("bash", ["-c", 'gh "$@"', "gh", ...args], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
    env: process.env,
    maxBuffer: 16 * 1024 * 1024,
  }).trim();
}

function parseJson(raw, label) {
  try {
    return JSON.parse(raw);
  } catch {
    throw new Error(`${label} query returned malformed JSON`);
  }
}

function createdAt(value, label) {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) throw new Error(`${label} has invalid created_at`);
  return timestamp;
}

function parseRecords(raw, label) {
  const parsed = parseJson(raw, label);
  if (!Array.isArray(parsed)) throw new Error(`${label} query returned non-array JSON`);
  // gh api --paginate --slurp 输出页数组；单页 mock 可能直接返回记录数组。
  return parsed.every(Array.isArray) ? parsed.flat() : parsed;
}

function latestByCreatedAt(items, label) {
  if (!Array.isArray(items)) throw new Error(`${label} query returned non-array JSON`);
  let latest = null;
  let latestTimestamp = -Infinity;
  for (const item of items) {
    if (!item || typeof item !== "object") throw new Error(`${label} contains malformed records`);
    const timestamp = createdAt(item.created_at, label);
    if (timestamp > latestTimestamp) {
      latest = item;
      latestTimestamp = timestamp;
    }
  }
  return latest;
}

function queryRun() {
  let raw;
  try {
    raw = gh(["run", "view", RUN_ID, "--repo", REPOSITORY, "--json", "status,conclusion,headSha"]);
  } catch {
    return { error: `run ${RUN_ID} query failed` };
  }
  let run;
  try {
    run = parseJson(raw, `run ${RUN_ID}`);
  } catch (error) {
    return { error: error.message };
  }
  if (!run || typeof run.status !== "string" || typeof run.headSha !== "string" || !run.headSha) {
    return { error: `run ${RUN_ID} query returned malformed data` };
  }
  if (run.conclusion !== null && typeof run.conclusion !== "string") {
    return { error: `run ${RUN_ID} query returned malformed conclusion` };
  }
  return { run };
}

function classifyRun(run) {
  if (run.headSha !== SHA) {
    return { state: "red", reason: `SHA mismatch: run ${RUN_ID} head ${run.headSha} != expected ${SHA}` };
  }
  const conclusion = run.conclusion || "-";
  if (RED_STATES.has(conclusion)) {
    return { state: "red", reason: `run ${RUN_ID} completed/${conclusion}` };
  }
  if (run.status === "completed") {
    if (conclusion === "success") return { state: "success", reason: `run ${RUN_ID} completed/success` };
    return { state: "red", reason: `run ${RUN_ID} completed/${conclusion}` };
  }
  if (WAIT_RUN_STATES.has(run.status)) {
    return { state: "wait", reason: `run ${RUN_ID} ${run.status}/${conclusion}` };
  }
  return { state: "red", reason: `run ${RUN_ID} returned unknown status ${run.status}` };
}

function queryDeployment() {
  let raw;
  try {
    raw = gh([
      "api", "--paginate", "--slurp",
      `repos/${REPOSITORY}/deployments?sha=${encodeURIComponent(SHA)}&environment=${encodeURIComponent(ENVIRONMENT)}&per_page=100`,
    ]);
  } catch {
    return {
      error: `deployment query failed (sha=${SHA.slice(0, 8)}, env=${ENVIRONMENT})`,
      deployment: { id: "", state: "unknown" },
    };
  }

  let latest;
  try {
    const deployments = parseRecords(raw, "deployment");
    const matching = deployments.filter(
      (item) => item?.sha === SHA && item?.environment === ENVIRONMENT,
    );
    latest = latestByCreatedAt(matching, "deployment");
  } catch (error) {
    return { error: error.message, deployment: { id: "", state: "unknown" } };
  }
  if (!latest) {
    return {
      deployment: { id: "", state: "none" },
      reason: `no deployment record (sha=${SHA.slice(0, 8)}, env=${ENVIRONMENT})`,
    };
  }
  if (typeof latest.id !== "string" && typeof latest.id !== "number") {
    return { error: "latest deployment has invalid id", deployment: { id: "", state: "unknown" } };
  }

  const deploymentId = String(latest.id);
  let statusRaw;
  try {
    statusRaw = gh([
      "api", "--paginate", "--slurp",
      `repos/${REPOSITORY}/deployments/${deploymentId}/statuses?per_page=100`,
    ]);
  } catch {
    return {
      error: `deployment ${deploymentId} status query failed`,
      deployment: { id: deploymentId, state: "unknown" },
    };
  }

  try {
    const statuses = parseRecords(statusRaw, `deployment ${deploymentId} status`);
    const latestStatus = latestByCreatedAt(statuses, `deployment ${deploymentId} status`);
    if (latestStatus && (typeof latestStatus.state !== "string" || !latestStatus.state)) {
      throw new Error("latest deployment status has invalid state");
    }
    const state = latestStatus?.state ?? "none";
    return {
      deployment: { id: deploymentId, state },
      reason: `deployment ${deploymentId} ${state}`,
    };
  } catch (error) {
    return {
      error: error.message,
      deployment: { id: deploymentId, state: "unknown" },
    };
  }
}

function classifyDeployment(result) {
  if (result.error) return { state: "wait", reason: result.error, deployment: result.deployment };
  const { deployment, reason } = result;
  if (deployment.state === "success") return { state: "success", reason, deployment };
  if (RED_STATES.has(deployment.state)) return { state: "red", reason, deployment };
  if (WAIT_DEPLOYMENT_STATES.has(deployment.state)) return { state: "wait", reason, deployment };
  return {
    state: "red",
    reason: `deployment ${deployment.id || "none"} returned unknown state ${deployment.state}`,
    deployment,
  };
}

function validPollInputs() {
  if (!/^\d+$/.test(TIMEOUT_TEXT) || !/^\d+$/.test(POLL_TEXT)) {
    return { error: "invalid timeout or poll-interval" };
  }
  return { timeout: Number(TIMEOUT_TEXT), pollInterval: Number(POLL_TEXT) };
}

function sleepSeconds(seconds) {
  if (seconds > 0) Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, seconds * 1000);
}

function waitForRun(poll) {
  const deadline = Date.now() + poll.timeout * 1000;
  while (true) {
    const queried = queryRun();
    if (queried.error) {
      if (Date.now() >= deadline) return red(queried.error);
      console.log(`${queried.error}; retry in ${poll.pollInterval}s`);
      sleepSeconds(poll.pollInterval);
      continue;
    }
    const verdict = classifyRun(queried.run);
    if (verdict.state === "success") {
      emit("conclusion", queried.run.conclusion);
      emit("head_sha", queried.run.headSha);
      console.log(`${verdict.reason} (sha=${queried.run.headSha.slice(0, 8)})`);
      return { state: "success", reason: verdict.reason };
    }
    if (verdict.state === "red") return red(verdict.reason);
    if (Date.now() >= deadline) return red(`timeout waiting for run ${RUN_ID} (${verdict.reason})`);
    console.log(`${verdict.reason}; retry in ${poll.pollInterval}s`);
    sleepSeconds(poll.pollInterval);
  }
}

function waitForDeployment(poll) {
  const deadline = Date.now() + poll.timeout * 1000;
  while (true) {
    const verdict = classifyDeployment(queryDeployment());
    if (verdict.state === "success") {
      emitDeployment(verdict.deployment);
      console.log(`${verdict.reason} (sha=${SHA.slice(0, 8)}, env=${ENVIRONMENT})`);
      return { state: "success", reason: verdict.reason, deployment: verdict.deployment };
    }
    if (verdict.state === "red") return red(verdict.reason, verdict.deployment);
    if (Date.now() >= deadline) {
      return red(`timeout waiting for deployment (${verdict.reason})`, verdict.deployment);
    }
    console.log(`${verdict.reason}; retry in ${poll.pollInterval}s`);
    sleepSeconds(poll.pollInterval);
  }
}

function gateVerdict(poll) {
  if (!RUN_ID) return red("run-id is required for ReleaseEvidence verdict");
  const deadline = Date.now() + poll.timeout * 1000;
  let runOk = false;
  let lastReason = "run pending";

  while (true) {
    if (!runOk) {
      const queried = queryRun();
      if (queried.error) {
        if (Date.now() >= deadline) return red(queried.error);
        console.log(`${queried.error}; retry in ${poll.pollInterval}s`);
        sleepSeconds(poll.pollInterval);
        continue;
      }
      const verdict = classifyRun(queried.run);
      if (verdict.state === "red") return red(verdict.reason);
      if (verdict.state === "success") {
        runOk = true;
        lastReason = verdict.reason;
        emit("conclusion", queried.run.conclusion);
        emit("head_sha", queried.run.headSha);
      } else {
        lastReason = verdict.reason;
      }
    }

    const deploymentVerdict = classifyDeployment(queryDeployment());
    lastReason = deploymentVerdict.reason;
    if (deploymentVerdict.state === "red") return red(lastReason, deploymentVerdict.deployment);
    if (runOk && deploymentVerdict.state === "success") {
      return green(
        `run ${RUN_ID} success + deployment ${deploymentVerdict.deployment.id} success (sha=${SHA.slice(0, 8)}, env=${ENVIRONMENT})`,
        deploymentVerdict.deployment,
      );
    }
    if (Date.now() >= deadline) {
      return red(
        `timeout waiting for ReleaseEvidence (run_ok=${runOk}, ${lastReason})`,
        deploymentVerdict.deployment,
      );
    }
    console.log(`ReleaseEvidence waiting (run_ok=${runOk}, ${lastReason}); retry in ${poll.pollInterval}s`);
    sleepSeconds(poll.pollInterval);
  }
}

function main() {
  if (!OUTPUT_FILE) throw new Error("GITHUB_OUTPUT is required");
  if (!SHA) throw new Error("INPUT_SHA is required");
  if (!REPOSITORY) throw new Error("GITHUB_REPOSITORY is required");
  if (!["waitForRun", "checkDeployment", "gateVerdict"].includes(MODE)) {
    throw new Error(`unknown ReleaseEvidence mode: ${MODE || "(missing)"}`);
  }
  const poll = validPollInputs();
  if (poll.error) {
    red(poll.error);
    process.exitCode = 1;
    return;
  }
  const result = MODE === "waitForRun"
    ? waitForRun(poll)
    : MODE === "checkDeployment"
      ? waitForDeployment(poll)
      : gateVerdict(poll);
  process.exitCode = result.state === "success" ? 0 : 1;
}

try {
  main();
} catch (error) {
  console.error(`::error::ReleaseEvidence failed closed: ${error.message}`);
  process.exitCode = 1;
}
