/**
 * #2388 · 增量 SSE 解析器的**行为级**守护（真执行，不读源码文本）
 *
 * ## 它在守什么
 *
 * 逐字输出换增量通道后，chunk 边界**没有任何对齐承诺**（RFC 9112 §7.1）——一个 chunk 可能
 * 含多帧、半帧，也可能切在 UTF-8 三/四字节字符中间。核心不变量是**同一字节流任意切分，
 * 事件序列一致**；其余口径（末帧不派发、终止即停、坏帧容错）都是为了让这条不变量在
 * 真实坏流下仍然成立。
 *
 * ## 为什么必须真跑
 *
 * 读源码文本只能证明「有一个 while 循环」，证明不了「`\r` 挂在块尾时下一块的 `\n`
 * 被吞掉」、「半截汉字没变成 U+FFFD」。故这里用 `utsHarness.loadUts` 把
 * `utils/sseIncrementalParser.uts` **真执行**：喂真字节（ArrayBuffer），断言真帧。
 *
 * 判据是**产出物**（帧序列 / `finish()` 三字段），不是字面量；harness 的 fail-closed
 * 保证模块新增/减少 import 或缺绑定会当场红。
 */
const path = require('path');

const { loadUts } = require('./utsHarness');

const PARSER_UTS = path.join(__dirname, 'sseIncrementalParser.uts');

/** 新建一个解析器实例（模块每次 loadUts 都是全新实例，模块级状态不串） */
function newParser() {
  const mod = loadUts(PARSER_UTS, {});
  return mod.createSseParser();
}

/** 字符串 → 真 ArrayBuffer（UTF-8 字节） */
function bytes(str) {
  const buf = Buffer.from(str, 'utf8');
  return buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength);
}

/** 把字节串按 `cuts` 指定的下标切成若干块（下标须递增、落在 0..len 内） */
function sliceBytes(buf, cuts) {
  const out = [];
  let prev = 0;
  for (const c of cuts) {
    out.push(buf.buffer.slice(buf.byteOffset + prev, buf.byteOffset + c));
    prev = c;
  }
  out.push(buf.buffer.slice(buf.byteOffset + prev, buf.byteOffset + buf.byteLength));
  return out;
}

/** 一次喂完整段文本，返回帧序列（JSON 化，便于整串比较） */
function feedAll(text, cuts) {
  const buf = Buffer.from(text, 'utf8');
  const chunks = cuts ? sliceBytes(buf, cuts) : [bytes(text)];
  const p = newParser();
  const frames = [];
  for (const c of chunks) {
    for (const f of p.feed(c)) frames.push(f);
  }
  return { frames, parser: p };
}

/** 帧序列的可比较形态（对象属性序无关） */
const shape = (frames) => frames.map((f) => ({ event: f.event, data: f.data }));

/** 完整一条 message 帧的字节文本 */
const msg = (payload) => 'event: message\ndata: ' + payload + '\n\n';

/** 终止帧 */
const doneFrame = 'event: done\ndata: null\n\n';

describe('#2388 增量 SSE 解析器', () => {
  describe('帧边界：末帧没有空行 ⇒ 不派发', () => {
    test('整帧齐备时按空行派发', () => {
      const { frames } = feedAll(msg('{"content":"甲"}'));
      expect(shape(frames)).toEqual([{ event: 'message', data: '{"content":"甲"}' }]);
    });

    test('末尾半帧（缺空行）不派发，finish 报 incomplete', () => {
      const text = msg('{"content":"甲"}') + 'event: message\ndata: {"content":"乙"}';
      const { frames, parser } = feedAll(text);
      expect(shape(frames)).toEqual([{ event: 'message', data: '{"content":"甲"}' }]);
      const fin = parser.finish();
      expect(fin.events).toEqual([]);
      expect(fin.terminated).toBe(false);
      expect(fin.incomplete).toBe(true);
    });

    test('末帧若是半截 done（缺空行）不会置 terminated —— #2310 的整体替换不被误触发', () => {
      const { parser } = feedAll('event: message\ndata: x\n\nevent: done\ndata: {"content":"全"}');
      const fin = parser.finish();
      expect(fin.terminated).toBe(false);
      expect(fin.incomplete).toBe(true);
      expect(fin.events).toEqual([]);
    });

    test('干净结束（末帧齐备且见过终止帧）finish 不报 incomplete', () => {
      const { parser } = feedAll(msg('{"content":"甲"}') + doneFrame);
      const fin = parser.finish();
      expect(fin.terminated).toBe(true);
      expect(fin.incomplete).toBe(false);
    });
  });

  describe('一 chunk 多帧：按帧序产出', () => {
    test('粘包里三帧顺序不变', () => {
      const { frames } = feedAll(msg('a') + msg('b') + msg('c'));
      expect(shape(frames)).toEqual([
        { event: 'message', data: 'a' },
        { event: 'message', data: 'b' },
        { event: 'message', data: 'c' },
      ]);
    });

    test('一帧里多行 data 以 \\n 连接', () => {
      const { frames } = feedAll('event: message\ndata: 第一行\ndata: 第二行\n\n');
      expect(shape(frames)).toEqual([{ event: 'message', data: '第一行\n第二行' }]);
    });
  });

  describe('字节边界：字段与行被切开也要拼回来', () => {
    test('event 行切成两块', () => {
      const { frames } = feedAll(msg('x'), [11]); // 'event: mess' | 'age\ndata: x\n\n'
      expect(shape(frames)).toEqual([{ event: 'message', data: 'x' }]);
    });

    test('空行本身切成两块', () => {
      const text = msg('x');
      const { frames } = feedAll(text, [text.length - 1]); // 最后只剩 '\n'
      expect(shape(frames)).toEqual([{ event: 'message', data: 'x' }]);
    });
  });

  describe('行尾：\\n / \\r\\n / \\r 三种都断行，且跨块正确', () => {
    test('CRLF 切在 \\r 与 \\n 之间（块尾挂起的 \\r 吞掉下一块首个 \\n）', () => {
      const text = 'event: message\r\ndata: x\r\n\r\n';
      const { frames } = feedAll(text, [14]); // 'event: message\r' | '\ndata: x\r\n\r\n'
      expect(shape(frames)).toEqual([{ event: 'message', data: 'x' }]);
    });

    test('孤立 \\r 断行（三帧）', () => {
      const { frames } = feedAll('event: message\rdata: a\r\revent: message\rdata: b\r\r');
      expect(shape(frames)).toEqual([
        { event: 'message', data: 'a' },
        { event: 'message', data: 'b' },
      ]);
    });

    test('CRLF 与 LF 混用', () => {
      const { frames } = feedAll('event: message\r\ndata: a\n\nevent: message\ndata: b\r\n\r\n');
      expect(shape(frames)).toEqual([
        { event: 'message', data: 'a' },
        { event: 'message', data: 'b' },
      ]);
    });
  });

  describe('UTF-8 字节面：任意切点都不出 U+FFFD', () => {
    const cases = [
      { name: '二字节 é', ch: 'é' },
      { name: '三字节 中', ch: '中' },
      { name: '四字节 😀', ch: '😀' },
    ];

    for (const c of cases) {
      test(c.name + '：逐一切点都与整块一致', () => {
        const text = msg(c.ch);
        const expectFrames = [{ event: 'message', data: c.ch }];
        const len = Buffer.from(text, 'utf8').length;
        for (let cut = 1; cut < len; cut += 1) {
          const { frames } = feedAll(text, [cut]);
          expect(shape(frames)).toEqual(expectFrames);
        }
      });
    }

    test('半截字符留在 carry：finish 不出占位符、只报 incomplete', () => {
      const text = msg('中文');
      const buf = Buffer.from(text, 'utf8');
      const cut = buf.byteOffset + buf.byteLength - 2; // 切掉最后一个字的 2 字节
      const p = newParser();
      const head = buf.buffer.slice(buf.byteOffset, cut);
      p.feed(head);
      const fin = p.finish();
      expect(fin.terminated).toBe(false);
      expect(fin.incomplete).toBe(true);
      // 承载面里没有任何 U+FFFD
      expect(JSON.stringify(fin).includes('\uFFFD')).toBe(false);
    });

    test('BOM 只在首块剥一次', () => {
      const text = '\uFEFF' + msg('x');
      for (const cut of [3, 4, 5, 6]) {
        const { frames } = feedAll(text, [cut]);
        expect(shape(frames)).toEqual([{ event: 'message', data: 'x' }]);
      }
    });
  });

  describe('坏帧容错：不认识的字段 / 缺 data / 超长行都不中断', () => {
    test('未知字段与注释行被忽略，帧照出', () => {
      const { frames } = feedAll(': 心跳注释\nid: 42\nretry: 3000\nevent: message\ndata: x\n\n');
      expect(shape(frames)).toEqual([{ event: 'message', data: 'x' }]);
    });

    test('缺 data 的帧 data 为空串（不抛）', () => {
      const { frames } = feedAll('event: status\n\n');
      expect(shape(frames)).toEqual([{ event: 'status', data: '' }]);
    });

    test('无字段名的空行连续出现不产出帧', () => {
      const { frames } = feedAll('\n\n\n');
      expect(shape(frames)).toEqual([]);
    });

    test('超长行整行丢弃，其后的帧照出', () => {
      const huge = 'x'.repeat(1024 * 1024 + 10);
      const { frames } = feedAll('data: ' + huge + '\nevent: message\ndata: ok\n\n');
      // 超长行导致该行的 data 被丢；event/data 仍按字段累积 ⇒ 帧出但 data 只有 'ok'
      expect(shape(frames)).toEqual([{ event: 'message', data: 'ok' }]);
    });
  });

  describe('终止帧：done / error 照常产出，之后不再产出', () => {
    test('done 之后喂什么都不再出帧，isTerminated 为真', () => {
      const p = newParser();
      const first = p.feed(bytes(msg('a') + doneFrame));
      expect(shape(first)).toEqual([
        { event: 'message', data: 'a' },
        { event: 'done', data: 'null' },
      ]);
      expect(p.isTerminated()).toBe(true);
      expect(shape(p.feed(bytes(msg('b'))))).toEqual([]);
      const fin = p.finish();
      expect(fin.terminated).toBe(true);
      expect(fin.incomplete).toBe(false);
    });

    test('error 同样终止', () => {
      const p = newParser();
      const out = p.feed(bytes('event: error\ndata: {"message":"超时"}\n\n'));
      expect(shape(out)).toEqual([{ event: 'error', data: '{"message":"超时"}' }]);
      expect(p.isTerminated()).toBe(true);
    });
  });

  describe('收尾幂等', () => {
    test('finish 调两次结果一致，且之后 feed 不产出', () => {
      const p = newParser();
      p.feed(bytes('event: message\ndata: x'));
      const a = p.finish();
      const b = p.finish();
      expect(shape(a.events)).toEqual([]);
      expect(b.terminated).toBe(a.terminated);
      expect(b.incomplete).toBe(a.incomplete);
      expect(shape(p.feed(bytes('\n\n')))).toEqual([]);
    });
  });

  describe('核心不变量：同一字节流任意切分，事件序列一致', () => {
    const stream =
      msg('{"content":"第一段"}') +
      'event: status\ndata: {"text":"思考中"}\n\n' +
      msg('{"content":"多字节😀中"}') +
      'event: usage\ndata: {"total":12}\n\n' +
      msg('{"content":"末段"}') +
      doneFrame;
    const expected = [
      { event: 'message', data: '{"content":"第一段"}' },
      { event: 'status', data: '{"text":"思考中"}' },
      { event: 'message', data: '{"content":"多字节😀中"}' },
      { event: 'usage', data: '{"total":12}' },
      { event: 'message', data: '{"content":"末段"}' },
      { event: 'done', data: 'null' },
    ];

    test('整体一次喂入', () => {
      expect(shape(feedAll(stream).frames)).toEqual(expected);
    });

    test('每个可能的单切点', () => {
      const len = Buffer.from(stream, 'utf8').length;
      for (let cut = 1; cut < len; cut += 1) {
        expect(shape(feedAll(stream, [cut]).frames)).toEqual(expected);
      }
    });

    test('等分切块（2..17 块）与伪随机切块都一致', () => {
      const len = Buffer.from(stream, 'utf8').length;
      for (let parts = 2; parts <= 17; parts += 1) {
        const cuts = [];
        for (let i = 1; i < parts; i += 1) cuts.push(Math.floor((len * i) / parts));
        expect(shape(feedAll(stream, cuts).frames)).toEqual(expected);
      }
      // 线性同余伪随机切法（种子固定 ⇒ 可复现）
      let seed = 20261109;
      const next = () => {
        seed = (seed * 1103515245 + 12345) % 2147483648;
        return seed;
      };
      for (let round = 0; round < 40; round += 1) {
        const k = 1 + (next() % 9);
        const picks = new Set();
        for (let i = 0; i < k; i += 1) picks.add(1 + (next() % (len - 1)));
        expect(shape(feedAll(stream, [...picks].sort((x, y) => x - y)).frames)).toEqual(expected);
      }
    });

    test('空块不破坏后续解析', () => {
      const p = newParser();
      const empty = new ArrayBuffer(0);
      expect(shape(p.feed(empty))).toEqual([]);
      expect(shape(p.feed(bytes(msg('a'))))).toEqual([{ event: 'message', data: 'a' }]);
      expect(shape(p.feed(empty))).toEqual([]);
    });
  });
});
