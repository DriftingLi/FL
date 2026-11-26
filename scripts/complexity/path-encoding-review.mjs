#!/usr/bin/env node
/**
 * ADR-0106 复评的机械两步：白名单查询 + 剔除已归批票号（#2904）。
 *
 * 只读：不建票、不改票、不碰仓库内容判红。ADR-0106 明记「不新增 label，不新增机检」，
 * 所以本脚本发现候选也只打印，恒以 0 退出；2 只留给「环境不可用 / 判据源解析失败」。
 *
 * 退出码
 *   0 = 正常产出读数行（候选多少都算正常，是否触发复评由人判）
 *   2 = gh 不可用、或 ADR-0106 里找不到 BATCHED_TICKETS 机读行（fail-closed：
 *       宁可失败，也不许静默输出未过滤清单）
 *
 * 用法
 *   node scripts/complexity/path-encoding-review.mjs
 *   node scripts/complexity/path-encoding-review.mjs --quarter 2026-Q3
 *   node scripts/complexity/path-encoding-review.mjs --json
 */
import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync } from "node:fs";
import { basename, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("../../", import.meta.url));

/** 白名单三词；--state all 与 in:title 在 ADR-0106 里是硬口径，这里同样写死、不给关掉的开关。 */
export const SEARCH = "中文路径 in:title OR quotepath in:title OR 乱码 in:title";

/** ADR-0106 基线行下的机读标记；票号写在同一行，避免脚本里出现中文正则。 */
export const BATCHED_MARKER = "BATCHED_TICKETS";

/** 根 docs/adr 下按编号前缀定位，文件名含中文也不硬写。 */
export const ADR_PREFIX = "ADR-0106-";

export const GH_JSON_FIELDS = "number,title,createdAt";

/** createdAt 是 UTC；季度也按 UTC 日历季算，换时区会挪边界，故不做本地化。 */
export function quarterOf(date) {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
    throw new TypeError("quarterOf: 需要有效的 Date");
  }
  return `${date.getUTCFullYear()}-Q${Math.floor(date.getUTCMonth() / 3) + 1}`;
}

/** 解析机读行；找不到标记或标记后没有票号都返回 null（调用方据此 fail-closed）。 */
export function parseBatched(text) {
  if (typeof text !== "string") return null;
  for (const line of text.split(/\r?\n/)) {
    const at = line.indexOf(BATCHED_MARKER);
    if (at < 0) continue;
    const numbers = line.slice(at + BATCHED_MARKER.length).match(/\d+/g);
    if (!numbers) return null;
    const unique = [...new Set(numbers.map(Number))]
      .filter((n) => Number.isInteger(n) && n > 0)
      .sort((a, b) => a - b);
    return unique.length > 0 ? unique : null;
  }
  return null;
}

/** 按季过滤 → 分开已归批与候选；createdAt 坏掉即抛（别静默当成候选）。 */
export function filterCandidates(issues, { quarter, batched }) {
  if (!Array.isArray(issues)) throw new TypeError("filterCandidates: issues 必须是数组");
  if (!Array.isArray(batched)) throw new TypeError("filterCandidates: batched 必须是票号数组");
  const known = new Set(batched.map(Number));
  const inQuarter = issues.filter((issue) => quarterOf(new Date(issue.createdAt)) === quarter);
  const kept = inQuarter.filter((issue) => known.has(Number(issue.number)));
  const candidates = inQuarter.filter((issue) => !known.has(Number(issue.number)));
  return { hits: inQuarter.length, batched: kept.length, candidates };
}

/** 读数行（key=value，便于贴进票的复评记录；候选另起一行便于人工通读）。 */
export function formatReading({ quarter, hits, batched, candidates, source }) {
  const line = `PATH_ENCODING_REVIEW quarter=${quarter} hits=${hits} batched=${batched} candidates=${candidates.length} source=${source}`;
  const detail = candidates.map(
    (issue) => `candidate #${issue.number} created=${issue.createdAt} title=${issue.title}`,
  );
  return { line, lines: [line, ...detail] };
}

export function parseIssuesJson(text) {
  const data = JSON.parse(text);
  if (!Array.isArray(data)) throw new TypeError("parseIssuesJson: gh 输出必须是数组");
  return data.map((item) => {
    if (!item || typeof item.number !== "number" || typeof item.createdAt !== "string") {
      throw new TypeError("parseIssuesJson: 票缺 number / createdAt");
    }
    return { number: item.number, title: String(item.title ?? ""), createdAt: item.createdAt };
  });
}

export function findAdr(adrDir) {
  const hits = readdirSync(adrDir).filter((name) => name.startsWith(ADR_PREFIX) && name.endsWith(".md"));
  if (hits.length !== 1) {
    throw new Error(`${adrDir} 下 ${ADR_PREFIX}*.md 命中 ${hits.length} 个（需恰好 1 个）`);
  }
  return join(adrDir, hits[0]);
}

function runGh(bin, search) {
  return execFileSync(
    bin,
    ["issue", "list", "--state", "all", "--search", search, "--json", GH_JSON_FIELDS, "--limit", "500"],
    { encoding: "utf8", maxBuffer: 32 * 1024 * 1024, stdio: ["ignore", "pipe", "pipe"] },
  );
}

export function parseArgs(argv) {
  const args = { quarter: null, json: false, adrDir: join(ROOT, "docs", "adr"), gh: "gh" };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--json") args.json = true;
    else if (arg === "--quarter") args.quarter = argv[++i] ?? null;
    else if (arg === "--adr-dir") args.adrDir = resolve(argv[++i] ?? "");
    else if (arg === "--gh") args.gh = argv[++i] ?? "gh";
    else throw new Error(`未知参数 ${arg}`);
  }
  return args;
}

export function main(argv) {
  let args;
  try {
    args = parseArgs(argv);
  } catch (error) {
    console.error(`PATH_ENCODING_REVIEW_ERROR args=${error.message}`);
    return 2;
  }

  let adrPath;
  let batched;
  try {
    adrPath = findAdr(args.adrDir);
    batched = parseBatched(readFileSync(adrPath, "utf8"));
  } catch (error) {
    console.error(`PATH_ENCODING_REVIEW_ERROR adr=${error.message}`);
    return 2;
  }
  if (!batched) {
    console.error("PATH_ENCODING_REVIEW_ERROR 判据源缺机读行 BATCHED_TICKETS（fail-closed，拒绝输出未过滤清单）");
    return 2;
  }

  const quarter = args.quarter ?? quarterOf(new Date());
  if (!/^\d{4}-Q[1-4]$/.test(quarter)) {
    console.error(`PATH_ENCODING_REVIEW_ERROR quarter=${quarter}（形态须为 YYYY-QN）`);
    return 2;
  }

  let issues;
  try {
    issues = parseIssuesJson(runGh(args.gh, SEARCH));
  } catch (error) {
    console.error(`PATH_ENCODING_REVIEW_ERROR gh=${error.message}`);
    return 2;
  }

  const result = filterCandidates(issues, { quarter, batched });
  const reading = formatReading({
    quarter,
    hits: result.hits,
    batched: result.batched,
    candidates: result.candidates,
    source: basename(adrPath),
  });
  if (args.json) {
    console.log(
      JSON.stringify(
        { quarter, hits: result.hits, batched: result.batched, candidates: result.candidates, source: adrPath },
        null,
        2,
      ),
    );
  } else {
    console.log(reading.lines.join("\n"));
  }
  return 0;
}

const entry = process.argv[1] ? resolve(process.argv[1]) : "";
if (entry === fileURLToPath(import.meta.url)) {
  process.exit(main(process.argv.slice(2)));
}
