/**
 * #2461 历年真题列表的展示判定守护（spec #2459；票 #2461 AC③④⑤）
 *
 * 被测对象是 `utils/realExamPapersDisplay.uts` —— 零运行期依赖的纯展示判定单点，
 * 这里用 `loadUts` 直接跑**真源码**，断言的都是页面看得见的文案与顺序：
 * 年份分组（无年份归「年份未知」、年份倒序）、每卷一行 meta、未解锁卷的价格行、
 * 空态文案、页头汇总文案。
 *
 * 为什么不断言页面：`.uvue` 的模板/样式在 node 里跑不了（`utsHarness` 的既定边界），
 * 而上面这几条正是页面唯一会出错的地方 —— 把它们收成纯函数是**为了能被这样测**
 * （与 `utils/jobCredentialDisplay.uts` 同形）。
 *
 * 最后一组是**变异取证**：把真源码改坏（年份比较器反向）后断言读数确实翻面，
 * 证明上面的「倒序」断言不是恒真。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const uts = require('./utsHarness');

const DISPLAY_UTS = path.join(__dirname, 'realExamPapersDisplay.uts');

/** 每次取全新模块实例（模块级状态不跨用例串） */
const display = () => uts.loadUts(DISPLAY_UTS, {});

/** 造一卷（字段齐备；patch 覆盖） */
function paper(patch) {
  return Object.assign({
    paper_id: 1,
    title: '2024 年叉车维修真题（卷一）',
    year: 2024,
    source: '内部真题',
    question_count: 50,
    duration_minutes: 60,
    entitled: false,
    price: 300,
  }, patch || {});
}

/** 变异：把真源码按 pairs 替换后写到临时目录，返回副本路径（先例 env 变异取证） */
function mutateDisplay(pairs) {
  let src = uts.readText(DISPLAY_UTS);
  for (const pair of pairs) {
    expect([pair[0], src.includes(pair[0])]).toEqual([pair[0], true]);
    src = src.split(pair[0]).join(pair[1]);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'real-exam-display-'));
  const file = path.join(dir, 'realExamPapersDisplay.uts');
  fs.writeFileSync(file, src);
  return file;
}

describe('groupRealExamPapers：年份分组与顺序（AC③）', () => {
  it('年份倒序，无年份的「年份未知」组垫底，同一年内保持入参顺序', () => {
    const { groupRealExamPapers } = display();
    const groups = groupRealExamPapers([
      paper({ paper_id: 1, year: 2023 }),
      paper({ paper_id: 2, year: 2025 }),
      paper({ paper_id: 3, year: 0, title: '未标注年份的卷' }),
      paper({ paper_id: 4, year: 2025 }),
      paper({ paper_id: 5, year: 2021 }),
    ]);
    expect(groups.map((g) => g.title)).toEqual(['2025', '2023', '2021', '年份未知']);
    expect(groups[0].papers.map((p) => p.paper_id)).toEqual([2, 4]);
    expect(groups[1].papers.map((p) => p.paper_id)).toEqual([1]);
    expect(groups[2].papers.map((p) => p.paper_id)).toEqual([5]);
    expect(groups[3].papers.map((p) => p.paper_id)).toEqual([3]);
  });

  it('一年多卷只出一组，卷数不丢', () => {
    const { groupRealExamPapers } = display();
    const groups = groupRealExamPapers([
      paper({ paper_id: 1, year: 2024 }),
      paper({ paper_id: 2, year: 2024 }),
      paper({ paper_id: 3, year: 2024 }),
    ]);
    expect(groups.length).toBe(1);
    expect(groups[0].papers.length).toBe(3);
  });

  it('空列表给空分组（页面靠长度判空态）', () => {
    const { groupRealExamPapers } = display();
    expect(groupRealExamPapers([])).toEqual([]);
  });
});

describe('paperMetaText：每卷一行 meta（AC③）', () => {
  it('来源 · N题 · M分钟', () => {
    const { paperMetaText } = display();
    expect(paperMetaText(paper({ source: '内部真题', question_count: 50, duration_minutes: 60 })))
      .toBe('内部真题 · 50题 · 60分钟');
  });

  it('来源为空回落「真题」，不留空档', () => {
    const { paperMetaText } = display();
    expect(paperMetaText(paper({ source: '', question_count: 20, duration_minutes: 30 })))
      .toBe('真题 · 20题 · 30分钟');
  });
});

describe('paperPriceText：未解锁卷的价格行（AC④）', () => {
  it('价格取自后端响应，端上不换算', () => {
    const { paperPriceText } = display();
    expect(paperPriceText(300)).toBe('300 积分/套');
    expect(paperPriceText(0)).toBe('0 积分/套');
  });
});

describe('papersEmptyText：空态文案（AC⑤）', () => {
  it('带当前证件名', () => {
    const { papersEmptyText } = display();
    expect(papersEmptyText('叉车维修（初级）')).toBe('叉车维修（初级） 真题建设中，敬请期待');
  });

  it('证件名没读到时不留半句空档', () => {
    const { papersEmptyText } = display();
    expect(papersEmptyText('')).toBe('真题建设中，敬请期待');
  });
});

describe('papersSummaryText：页头汇总（AC②）', () => {
  it('当前证件 + 卷总数', () => {
    const { papersSummaryText } = display();
    expect(papersSummaryText('叉车维修（初级）', 8)).toBe('当前证件：叉车维修（初级） · 共 8 套');
  });

  it('证件名没读到用破折号占位', () => {
    const { papersSummaryText } = display();
    expect(papersSummaryText('', 0)).toBe('当前证件：— · 共 0 套');
  });
});

describe('paperRedeemButtonText：解锁按钮文案与在途态（#2462 AC①④）', () => {
  it('未解锁卷显示「积分解锁」', () => {
    const { paperRedeemButtonText } = display();
    expect(paperRedeemButtonText(false)).toBe('积分解锁');
  });

  it('在途时换成「兑换中...」（按钮组件没有 loading 态，靠文案 + disabled 表达）', () => {
    const { paperRedeemButtonText } = display();
    expect(paperRedeemButtonText(true)).toBe('兑换中...');
    expect(paperRedeemButtonText(true)).not.toBe(paperRedeemButtonText(false));
  });
});

describe('paperRedeemConfirmText：二次确认必须含该卷价格（#2462 AC①）', () => {
  it('价格原样来自后端响应，端上不换算、不硬编码', () => {
    const { paperRedeemConfirmText } = display();
    expect(paperRedeemConfirmText(300)).toContain('300');
    expect(paperRedeemConfirmText(500)).toContain('500');
    expect(paperRedeemConfirmText(500)).not.toContain('300');
  });

  it('说清解锁后的后果（可开始练习与模拟考试）', () => {
    const { paperRedeemConfirmText } = display();
    expect(paperRedeemConfirmText(300)).toBe('该真题卷需 300 积分，解锁后可开始练习与模拟考试，确认兑换？');
  });
});

describe('paperRedeemFailText：后端原因 + 重拉后的真实状态（#2462 AC③⑤）', () => {
  it('后端哨兵原因原样透传（不另造话术，也不作文案比对）', () => {
    const { paperRedeemFailText } = display();
    expect(paperRedeemFailText('积分不足', true).indexOf('积分不足')).toBe(0);
    expect(paperRedeemFailText('网络异常，请稍后重试', true).indexOf('网络异常，请稍后重试')).toBe(0);
  });

  it('仍未解锁 / 已解锁两句话由重拉结果决定', () => {
    const { paperRedeemFailText } = display();
    expect(paperRedeemFailText('积分不足', true)).toContain('该卷仍未解锁');
    expect(paperRedeemFailText('已兑换', false)).toContain('该卷已解锁');
    expect(paperRedeemFailText('已兑换', false)).not.toContain('仍未解锁');
  });

  it('同一原因换个状态就换半句：判据是 entitled 而不是失败文案', () => {
    const { paperRedeemFailText } = display();
    expect(paperRedeemFailText('已兑换', true)).not.toBe(paperRedeemFailText('已兑换', false));
  });
});

describe('变异取证：倒序断言必须有牙', () => {
  it('年份比较器反向 ⇒ 读数是升序（与上面判据相反）', () => {
    const mutated = mutateDisplay([['return b.year - a.year', 'return a.year - b.year']]);
    const { groupRealExamPapers } = uts.loadUts(mutated, {});
    const titles = groupRealExamPapers([
      paper({ paper_id: 1, year: 2023 }),
      paper({ paper_id: 2, year: 2025 }),
    ]).map((g) => g.title);
    expect(titles).toEqual(['2023', '2025']);
  });
});

describe('变异取证：#2462 解锁文案的断言必须有牙', () => {
  it('确认文案丢掉价格 ⇒ 价格断言翻面', () => {
    const mutated = mutateDisplay([["price.toString() + ' 积分，解锁后", "'' + ' 积分，解锁后"]]);
    const { paperRedeemConfirmText } = uts.loadUts(mutated, {});
    expect(paperRedeemConfirmText(300)).not.toContain('300');
  });

  it('在途文案塌回「积分解锁」⇒ 在途/空闲两态不可分（AC④ 失守）', () => {
    const mutated = mutateDisplay([["= '兑换中...'", "= '积分解锁'"]]);
    const { paperRedeemButtonText } = uts.loadUts(mutated, {});
    expect(paperRedeemButtonText(true)).toBe(paperRedeemButtonText(false));
  });

  it('失败提示丢掉「仍未解锁」⇒ 学员看不出下一步（AC③ 失守）', () => {
    const mutated = mutateDisplay([["'，该卷仍未解锁，可稍后重试。'", "''"]]);
    const { paperRedeemFailText } = uts.loadUts(mutated, {});
    expect(paperRedeemFailText('积分不足', true)).toBe('积分不足');
  });
});
