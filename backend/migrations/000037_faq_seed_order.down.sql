-- 回滚 #1079 的顺序修正：把这 30 条种子的 sort_order 置回 000036 的 0。
-- （同样按 (分类 code, 问题原文) 定位，改过文本的行不动。）
UPDATE faq f
   SET sort_order = 0
  FROM (VALUES
    ('account', '怎么注册学员账号？'),
    ('account', '忘记密码怎么办？'),
    ('account', '修改密码后，其他设备会掉线吗？'),
    ('account', '怎么修改昵称和头像？'),
    ('account', '怎么注销账号？注销后数据还在吗？'),
    ('credential', '「当前证件」是什么？'),
    ('credential', '切换证件后，以前的练习记录还在吗？'),
    ('credential', '什么时候需要重新选定证件？'),
    ('course', '课程进度是怎么算的？'),
    ('course', '学习资料在哪里看？'),
    ('course', '收藏的内容在哪里找？'),
    ('course', '学习时长是怎么统计的？'),
    ('exam', '练习模式有哪几种？'),
    ('exam', '模拟考试和真题练习有什么区别？'),
    ('exam', '真题卷怎么解锁？'),
    ('exam', '错题是怎么被收录的？'),
    ('exam', '错题本里的「最近错误时间」和「我上次选的答案」是什么？'),
    ('exam', '题目解析是 AI 生成的吗？'),
    ('points', '积分怎么获得？'),
    ('points', '每日任务什么时候重置？'),
    ('points', '「每日登录」需要先做什么吗？'),
    ('points', '每日打卡的连击奖励怎么算？'),
    ('points', '积分能用来做什么？'),
    ('note', '笔记记在哪里？'),
    ('note', '同一道题可以记几条笔记？'),
    ('note', '论坛发帖和回复有积分奖励吗？'),
    ('note', '帖子被加精或被认定为备考经验有奖励吗？'),
    ('job', '简历怎么填？'),
    ('job', '投递后企业什么时候能看到我的联系方式？'),
    ('job', '怎么撤回投递？')
  ) AS v(cat, question)
  JOIN faq_category c ON c.code = v.cat
 WHERE f.category_id = c.id
   AND f.question = v.question;
