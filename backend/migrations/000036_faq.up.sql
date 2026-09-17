-- #1079：帮助中心（FAQ）新域。
--
-- 学员遇到「证件怎么切」「积分怎么来」「打卡断了会不会清零」这类问题无处可查——全仓此前
-- 零 FAQ 资产。两张表：分类（可排序 / 可停用）+ 条目（关联分类）。
--
-- 口径（grilling 定案）：
--   - **只有已发布 / 停用两态**，不做草稿工作流；
--   - answer 是**纯文本、原样换行**渲染，不引入 Markdown 的内容渲染口径（ADR-0046）；
--   - 学员面一次返回「分类 + 全部已发布条目」，搜索走端上过滤——不新增搜索接口、
--     不进全局搜索域（ADR-0049）。
--
-- 种子内容：7 类共 30 条，**每条只写代码里可验证的真实行为**（不编功能）。种子只在
-- 全新库落一次（`WHERE NOT EXISTS (SELECT 1 FROM faq)`）——已有内容的库不重复灌入，
-- 管理员删改过的条目也不会被迁移重新塞回来。

CREATE TABLE IF NOT EXISTS faq_category (
    id         SERIAL PRIMARY KEY,
    code       VARCHAR(64) NOT NULL UNIQUE,
    title      TEXT        NOT NULL,
    sort_order INT         NOT NULL DEFAULT 0,
    enabled    BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE faq_category IS '帮助中心分类（#1079）：学员端左侧分类的可排序 / 可停用清单';
COMMENT ON COLUMN faq_category.code IS '分类稳定标识（小写字母/数字/下划线）：管理端与日志引用它，不引用自增 id';
COMMENT ON COLUMN faq_category.enabled IS '停用后学员端不再展示；条目保留，管理端仍可见可改';

CREATE TABLE IF NOT EXISTS faq (
    id          SERIAL PRIMARY KEY,
    category_id INT         NOT NULL REFERENCES faq_category(id) ON DELETE CASCADE,
    question    TEXT        NOT NULL,
    answer      TEXT        NOT NULL,
    sort_order  INT         NOT NULL DEFAULT 0,
    published   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_faq_category ON faq (category_id, sort_order, id);

COMMENT ON TABLE faq IS '帮助中心条目（#1079）：已发布 / 停用两态，无草稿工作流';
COMMENT ON COLUMN faq.answer IS '纯文本、原样换行渲染；不引入 Markdown 渲染口径';
COMMENT ON COLUMN faq.category_id IS '所属分类；ON DELETE CASCADE —— 删分类会连带删除其下全部条目';

INSERT INTO faq_category (code, title, sort_order, enabled) VALUES
  ('account',    '账号与登录',      1, true),
  ('credential', '证件与学习路径',  2, true),
  ('course',     '课程学习',        3, true),
  ('exam',       '题库与考试',      4, true),
  ('points',     '积分与任务',      5, true),
  ('note',       '笔记与社区',      6, true),
  ('job',        '就业与简历',      7, true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO faq (category_id, question, answer, sort_order, published)
SELECT c.id, v.question, v.answer, v.sort_order, true
  FROM (VALUES
    ('account', '怎么注册学员账号？', '在登录页点「注册」，可用手机号或邮箱验证码注册。注册完成后需要在引导页选定一个目标证件，课程、题库、模考才会按该证件展示内容。', 0),
    ('account', '忘记密码怎么办？', '在登录页点「忘记密码」，用注册时绑定的手机号或邮箱接收验证码后重设密码。', 0),
    ('account', '修改密码后，其他设备会掉线吗？', '会。改密会吊销该账号的全部刷新令牌，其他设备在下一次请求时就需要重新登录。', 0),
    ('account', '怎么修改昵称和头像？', '在「个人资料」页提交修改。昵称与头像走资料审核，审核通过后才对外生效；审核期间页面会显示「审核中」。', 0),
    ('account', '怎么注销账号？注销后数据还在吗？', '在「个人资料」页发起注销。注销会一并清除学习记录、练习记录、错题本、笔记、打卡记录、站内信等个人数据，不可恢复。', 0),
    ('credential', '「当前证件」是什么？', '它是你备考的目标证件，显示在侧栏顶部的证件切换器上。课程、题库、练习、模考、错题本、搜索与收藏都按它过滤；论坛与 AI 助手不按证件过滤。', 0),
    ('credential', '切换证件后，以前的练习记录还在吗？', '在。练习记录、练习进度与模考记录会在产生的那一刻记下当时的证件，之后切换证件不会改写历史归属。切到没练过的证件会看到空历史，那是空态，不是数据丢失。', 0),
    ('credential', '什么时候需要重新选定证件？', '换方向备考时随时可以切换。切换会立即生效：所有按证件过滤的页面自动刷新并回到第一页。', 0),
    ('course', '课程进度是怎么算的？', '按章节学习进度累计。章节进度达到 100% 记为完成，课程进度由它的章节汇总而来。', 0),
    ('course', '学习资料在哪里看？', '侧栏「学习资料」会把各课程挂载的附件聚合到一起，可直接预览或下载。', 0),
    ('course', '收藏的内容在哪里找？', '侧栏「我的收藏」。课程、章节、题目、资讯与帖子都可以收藏，统一在那一页查看。', 0),
    ('course', '学习时长是怎么统计的？', '按章节学习的时长累计，「个人资料」页显示的「学习时长」就是它的汇总。', 0),
    ('exam', '练习模式有哪几种？', '顺序练习、随机练习、专项练习与标签练习四种。它们共用同一套判分与解析。', 0),
    ('exam', '模拟考试和真题练习有什么区别？', '模拟考试按当前证件抽题组卷，交卷后给出成绩、正确率与易错项统计；真题练习是按套卷作答的真题。', 0),
    ('exam', '真题卷怎么解锁？', '在积分商城用积分兑换，当前为 300 分一套。兑换后该套卷永久可做。', 0),
    ('exam', '错题是怎么被收录的？', '练习与模拟考试中答错的题会按题自动收录进错题本。在错题本里可以收藏、重做或移出；重做答对会自动移出错题本。', 0),
    ('exam', '错题本里的「最近错误时间」和「我上次选的答案」是什么？', '「最近错误时间」是这道题最近一次答错的时间；「我上次选的答案」是你最近一次作答提交的答案，用来和正确答案对照。', 0),
    ('exam', '题目解析是 AI 生成的吗？', '解析在需要时生成并缓存，之后复用；如果站点没有配置 AI，会降级显示题库里的静态解析。', 0),
    ('points', '积分怎么获得？', '主要有四条路：任务中心的每日任务领取、每日打卡直记、论坛发帖与回复他人主题、投稿过审与下载量达阶奖励。', 0),
    ('points', '每日任务什么时候重置？', '每天 0 点（北京时间）重置，每项任务当日各可领取一次。', 0),
    ('points', '「每日登录」需要先做什么吗？', '不需要。进入任务中心即可直接领取，每天一次。', 0),
    ('points', '每日打卡的连击奖励怎么算？', '基础每天 5 分；连续签到满 3 / 7 / 30 天时，当天额外加 5 / 10 / 50 分。断签后连续天数清零，没有补签。', 0),
    ('points', '积分能用来做什么？', '可以兑换课程、真题套卷与商城商品。每一次获得和消耗都在「积分明细」里有记录。', 0),
    ('note', '笔记记在哪里？', '在题目里记的笔记和「我的笔记」页看到的是同一份数据。除了挂在题目上的笔记，你也可以在「我的笔记」里新建与题目无关的独立笔记。', 0),
    ('note', '同一道题可以记几条笔记？', '一条。对同一道题重复保存是覆盖更新，不会新增第二条。独立笔记则没有条数限制。', 0),
    ('note', '论坛发帖和回复有积分奖励吗？', '有。回复类任务只统计回复「他人主题」的次数，自己回复自己的帖子不计分。', 0),
    ('note', '帖子被加精或被认定为备考经验有奖励吗？', '有，且两者合并为同一笔奖励发放。认定由管理员授予，学员自己标注不算。', 0),
    ('job', '简历怎么填？', '侧栏「我的简历」。证书信息会从你已上传并通过审核的证件资料带出，无需重复填写。', 0),
    ('job', '投递后企业什么时候能看到我的联系方式？', '企业需要先发起联络申请，经你同意后才会看到；未同意之前，企业只能看到脱敏后的简历卡。', 0),
    ('job', '怎么撤回投递？', '在「我的投递」里撤回。撤回会连带撤销你已经给出的联络授权。', 0)
  ) AS v(cat, question, answer, sort_order)
  JOIN faq_category c ON c.code = v.cat
 WHERE NOT EXISTS (SELECT 1 FROM faq);

