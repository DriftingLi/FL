# ADR-0111：部署拓扑单一 owner 与机器可读快照

- 状态：已接受
- 日期：2026-11-27
- 领域：部署 / CI/CD / 巡检 / 备份

## 背景

外部 PostgreSQL 引入后，`DB_MODE` 的解释分散在 `deploy.py`、CD workflow、compose overlay、watchdog 与每日备份中。每增加一种数据库模式，都需要跨多个 implementation 同步容器、迁移、备份和巡检策略，已经形成多 owner 漂移面。

## 决策

1. **部署拓扑由 Python 部署 module 单一解析**。它消费 EffectiveConfig 与环境输入，产出 compose 组合、期望容器、迁移入口、备份与巡检策略。
2. **docker 与 external 是同一 interface 后的两个真实 adapter**；workflow 与 shell 不自行解释 `DB_MODE`。
3. **部署成功后原子发布非敏感拓扑快照**。快照带发布标识，包含当前拓扑结论，不包含密钥。
4. **watchdog 与每日备份只消费快照**。缺失、结构损坏、发布标识不匹配或必需字段缺席即 fail closed。

## 后果

- 拓扑修改集中在一个 module，测试面可从真实解析结果覆盖 CLI、compose、巡检与备份组合。
- 新增快照产物需要随发布计划一起校验版本与原子替换，避免半写文件被消费。
- EffectiveConfig 仍只负责配置取值优先级，不承担拓扑策略；ReleaseTransaction 仍只负责应用发布生命周期。

## 关联

- ADR-0088：部署面聚合与按变更重启
- ADR-0112：版本化发布计划与应用回滚
- `CONTEXT.md`：部署拓扑、部署拓扑快照、EffectiveConfig
