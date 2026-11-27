import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, rmSync } from "node:fs";
import { readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import {
  BATCHED_MARKER,
  filterCandidates,
  findAdr,
  formatReading,
  parseBatched,
  parseIssuesJson,
  quarterOf,
} from "./path-encoding-review.mjs";

const ROOT = fileURLToPath(new URL("../../", import.meta.url));

test("季度按 UTC 日历季切，跨季边界不许漂", () => {
  assert.equal(quarterOf(new Date("2026-03-31T23:59:59Z")), "2026-Q1");
  assert.equal(quarterOf(new Date("2026-04-01T00:00:00Z")), "2026-Q2");
  assert.equal(quarterOf(new Date("2026-11-26T20:10:00Z")), "2026-Q4");
  assert.equal(quarterOf(new Date("2026-12-31T23:59:59Z")), "2026-Q4");
  assert.equal(quarterOf(new Date("2027-01-01T00:00:00Z")), "2027-Q1");
  assert.throws(() => quarterOf(new Date("not-a-date")), TypeError);
});

test("机读行解析：取标记之后的票号，去重升序", () => {
  const text = "基线行\n已归批票号：BATCHED_TICKETS 2875 1853 2875 2893\n后果段 12 处";
  assert.deepEqual(parseBatched(text), [1853, 2875, 2893]);
});

test("机读行缺失或空 ⇒ null（调用方据此 fail-closed）", () => {
  assert.equal(parseBatched("只有基线，没有机读行"), null);
  assert.equal(parseBatched(`已归批票号：${BATCHED_MARKER}（还没填）`), null);
  assert.equal(parseBatched(""), null);
  assert.equal(parseBatched(undefined), null);
});

test("按季过滤并剔除已归批：hits / batched / candidates 三项对齐", () => {
  const issues = [
    { number: 2875, title: "ADR 落地", createdAt: "2026-11-26T03:44:22Z" },
    { number: 2902, title: "召回补丁", createdAt: "2026-11-26T11:48:11Z" },
    { number: 1027, title: "上季事故", createdAt: "2026-09-15T02:00:00Z" },
  ];
  const result = filterCandidates(issues, { quarter: "2026-Q4", batched: [1853, 2875] });
  assert.equal(result.hits, 2);
  assert.equal(result.batched, 1);
  assert.deepEqual(result.candidates.map((i) => i.number), [2902]);
});

test("createdAt 坏掉不许静默当候选", () => {
  assert.throws(
    () => filterCandidates([{ number: 1, title: "", createdAt: "坏时间" }], { quarter: "2026-Q4", batched: [] }),
    TypeError,
  );
});

test("gh 输出必须是数组且带 number / createdAt", () => {
  assert.deepEqual(parseIssuesJson('[{"number":1,"title":"t","createdAt":"2026-11-26T00:00:00Z"}]'), [
    { number: 1, title: "t", createdAt: "2026-11-26T00:00:00Z" },
  ]);
  assert.throws(() => parseIssuesJson("{}"), TypeError);
  assert.throws(() => parseIssuesJson('[{"title":"缺号"}]'), TypeError);
});

test("读数行是 key=value，候选逐行列出", () => {
  const reading = formatReading({
    quarter: "2026-Q4",
    hits: 4,
    batched: 4,
    candidates: [],
    source: "ADR-0106-x.md",
  });
  assert.equal(
    reading.line,
    "PATH_ENCODING_REVIEW quarter=2026-Q4 hits=4 batched=4 candidates=0 source=ADR-0106-x.md",
  );
  const withCandidate = formatReading({
    quarter: "2026-Q4",
    hits: 1,
    batched: 0,
    candidates: [{ number: 2902, title: "新票", createdAt: "2026-11-26T11:48:11Z" }],
    source: "ADR-0106-x.md",
  });
  assert.equal(withCandidate.lines.length, 2);
  assert.match(withCandidate.lines[1], /candidate #2902/);
});

test("真实 ADR-0106 可定位、机读行可解析且覆盖收口批", () => {
  const adrPath = findAdr(join(ROOT, "docs", "adr"));
  const batched = parseBatched(readFileSync(adrPath, "utf8"));
  assert.ok(batched, `${adrPath} 缺 ${BATCHED_MARKER} 机读行`);
  assert.ok(batched.length >= 8, "已归批清单至少覆盖基线那 8 张");
  for (const ticket of [1853, 2139, 2873, 2893]) {
    assert.ok(batched.includes(ticket), `已归批清单缺 #${ticket}`);
  }
});

test("ADR 目录里找不到唯一前缀 ⇒ 抛（不许猜文件名）", () => {
  const empty = mkdtempSync(join(tmpdir(), "pe-review-"));
  try {
    assert.throws(() => findAdr(empty), /命中 0 个/);
  } finally {
    rmSync(empty, { recursive: true, force: true });
  }
});
