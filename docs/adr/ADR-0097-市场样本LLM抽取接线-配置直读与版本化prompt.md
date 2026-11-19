# ADR-0097：市场样本 LLM 抽取接线——配置直读 + 版本化 prompt + 抽检闸门

- 状态：Accepted（2026-11-19）
- 日期：2026-11-19
- 领域：残值评估 / 数据采集（市场真值）
- 票：#2673（实施）· 父规格 #2671 · 承接 ADR-0096

## 背景

ADR-0096 立起的 tracer bullet 只落了 raw 行：渠道 / 来源 URL / 原始描述 jsonb 三项为事实留底，其余字段（结构化规格、价格、置信度、抽取版本）全部空着。这些字段是「市场真值 → 弱标签」链路的第一道加工：公告正文是非结构化文本，必须经 LLM 抽取成结构化规格与原始价格，才可能被后续清洗/匹配成残值率标签。#2673 把这道加工接线，并回答三个问题：抽取配置从哪来、怎么知道一行样本可不可信、模型输出的版本怎么追踪。

## 决策

**一、复用现有 AI 配置与特征绑定机制，collector 同 PG 网络直读。** 新增 `feature_key=market_sample_extract`（Go 注册表 `FeatureMarketSampleExtract`，`billed=false`、无 slug、非对话），管理端首次配置一个 `ai_configs` 条目并绑定到该 key；collector batch 在**同一个 PG 连接/事务**内查 `ai_feature_bindings → ai_configs`，用与后端 `SECRET_KEY` 相同的密钥解密 `api_key`（Python 镜像与 `internal/security/crypto.go` 的 `enc:v1:` / AES-256-GCM 格式逐字节对齐，有跨语言向量测试钉住）。不新造配置表，不引入第二套密钥体系。后端注册表只提供 isValidFeature 准入与展示标签，真实 prompt 不在后端。

**二、prompt 版本化，且版本即写回字段。** 抽取 prompt 收敛为 `collector/extract/prompt.py` 的 `PROMPT_VERSION`（当前 `v1`）；同一常量既嵌进 system message，又写进样本的 `extraction_version`。这样「哪条样本是哪个 prompt 版本抽的」永远同源，不会出现代码换了 prompt、样本标着旧版本的漂移。

**三、抽取结果写回同一 raw 行，原文留底不动。** 抽取输出（结构化规格、逐字段置信度、原始价格及类型、评估价）写进 `market_transactions` 已有字段；`channel` / `source_url` / `raw_description` 三个原始事实列**永不覆盖**（延续 ADR-0096 的不可变留底）。价格存原始挂牌/成交价，仍不施加清算折扣 δ（ADR-0096 决策四不变）。

**四、低置信与价格失败各留「可查对象」——抽检队列 + 标签资格。** 迁移 `000064` 新增 `market_transactions.label_eligible` 与一等表 `market_sample_review_queue(sample_id, reason, detail)`：
- 关键字段（brand/model/series/tonnage/year/engine_hours/raw_price）任一置信度**缺失或 < 0.75** ⇒ 写一行 `reason=low_confidence`，detail 列记下具体低置信字段（JSON）；
- 价格抽取失败（raw_price 为空或置信度不达标）⇒ `label_eligible=false` 并写一行 `reason=price_extraction_failed`。

阈值 0.75 由用户在会话确认，落为具名常量 `LOW_CONFIDENCE_THRESHOLD` 并写入本 ADR。用独立队列表而非 jsonb 标记位，是为了让「抽检队列对象」可索引、可聚合、有稳定 id 供下游消费。标签资格与队列写入在**同一事务**，避免「标了不可标签却没进队列」的半状态。

**五、契约测试用录制 fixture 打桩，不测 prompt 质量。** 单一 seam 仍是「fixture HTML 进 → 真 PG 行出」（ADR-0096 决策三）；抽取点 `extractor` 是可注入参数，测试注入从 JSON fixture 读出的确定性输出，断言配置确实被读取并解密、字段写回正确、队列与标签资格符合判定；真实 LLM 调用（`urllib` OpenAI 兼容 chat）只在生产路径用。测试不调用模型、不评估抽取质量——那属于 prompt 工程，不是契约。

## 被否备选

- **为抽取单建配置表 / 单造密钥**：能少依赖后端机制，但会出现两套 api_key 管理与两套密钥轮换；复用 `ai_configs` + `SECRET_KEY` 让配置管理与解密口径全仓唯一。
- **把 prompt 放进后端注册表 systemPrompt 字段**：看似「配置集中」，但 collector 是独立进程、抽取 prompt 与对话提示词生命周期不同，塞进注册表只会让 regeneration 契约（`features_codegen_test`）无谓变红；真实 prompt 归 collector 版本化，后端只留占位声明满足注册表卫生。
- **低置信只写 jsonb 标记位、不建队列表**：改动更小，但「抽检队列对象可查」退化成全表扫 jsonb，无法按 reason 聚合、无稳定 id；spec 明确要队列对象。
- **价格失败不单独标记，靠 raw_price is null 推断**：下游要重复判定逻辑且无法区分「抽失败」与「公告本就没价格」；显式 `label_eligible` + 队列原因消除歧义。
- **契约测试打真实模型**：引入网络不确定性与成本，且 prompt 质量不是本票验收项；注入 stub 才能确定性断言写回/队列逻辑。

## 后果

**收益**：①市场公告第一次能自动转成结构化规格与原始价格，弱标签链路有了可加工的原料；②配置直读复用全仓唯一机制，生产首次配置走既有 admin API，无新运维面；③每条样本的可信度（置信度 + 标签资格 + 队列原因）与抽取版本都有据可查，下游校准可据此加权/排除；④跨语言加密向量测试证明 Python 解密与后端加密真实互操作。

**代价**：①新增 `cryptography` Python 依赖（仅解密用，面很窄）；②多一个 `market_sample_review_queue` 表与 `label_eligible` 列，读方需按标签资格/清洗状态过滤（raw 且不可标签的行绝不能进标签集）；③低置信阈值 0.75 是初版经验值，后续可据抽检反馈调整——调参需升 `PROMPT_VERSION` 或另立规则版本，避免已有样本判定口径 silently 改变；④collector 容器新增 `SECRET_KEY` 环境变量（与后端共用同一把，无新密钥）。

**实施边界（本票不做）**：清洗七规则版本化模块、分层匹配器（7→4→3）、残值率标签计算、校准拟合、抽检队列的消费/复核 UI、daemon 增量模式——全部在后续票（#2674 起）。本票只交付「抽取 → 写回 → 标记/入队」这一段。
