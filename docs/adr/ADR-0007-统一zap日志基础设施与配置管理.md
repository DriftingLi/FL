# ADR-0007: 统一 zap 日志基础设施与 Viper 配置管理

- 状态：已接受（2026-08-08，spec #61 / tickets #62~#70）
- 领域：后端基础设施 —— 系统运行日志与配置加载

## 背景

后端此前没有统一的「系统运行日志（app log）」体系：

- 主业务（api/service/middleware）散落使用 Go 标准库 `log/slog`；残值评估子模块单独使用 zap——两套日志栈并存，格式、字段、脱敏策略互不相同；
- 生产日志仅依赖 docker json-file driver 轮转（max-size/max-file），容器重建即丢，无文件持久化、无压缩备份、无保留策略；
- 无敏感信息防护：错误串可能携带连接串凭证（如 DATABASE_URL），日志落盘后 PII/凭证不可控；
- 访问日志缺 request_id / user_id / user_role 字段，健康检查探活每 30s 刷屏淹没真实请求；
- 配置为手写 `os.Getenv` + strconv 样板（约 400 行），约 30 处默认值散落在绑定调用中，无集中管理。

## 决策

### 1. zap 全后端统一，`internal/logger` 为唯一日志栈

- 新建 `internal/logger` 模块：工厂（级别/console|json 编码器/stdout|文件输出）、脱敏、访问日志中间件；
- 主业务全部构造签名追加 `*zap.Logger` 参数，以**构造注入**分发（无全局访问器）；测试注入 `zap.NewNop()`；唯一例外：`internal/cache` 为包级函数形态（无持有 logger 的对象），经 `InitRedis(cfg, logger)` 在装配根注入包级 logger（与全局 `client` 同一注入模式）；
- 日志器以 `zapcore.AtomicLevel` 构建，支持运行时调整级别（生产事故排查无需重启）；
- 脱敏经包装 core 对**所有输出字段**统一生效：敏感 key 打码、Error 字段错误串内嵌连接串凭证过滤；
- 残值评估子模块旧有独立日志器退役（`VALUATION_LOG_*` 配置移除），收敛到同一实例；
- GORM 经自定义适配器接入 zap，保持级别语义（错误→Error、慢查询→Warn，`LOG_LEVEL=warn/error` 时 GORM 错误不被吞掉）；
- 选 zap 弃 slog 的理由：本项目已有 zap 依赖与注入式先例（valuation 子模块），slog 无开箱的 field 脱敏与 lumberjack 生态配合；两套栈并存比单一栈代价更高。

### 2. 生产日志：文件持久化 + 轮转即备份/清理 + stdout 双写

- 生产 `LOG_DIR` 挂卷（`/data/logs`）写文件，lumberjack 按大小轮转、压缩归档、保留份数/天数上限（默认 100MB/7 份/30 天，全部 env 可覆盖）——「备份」= 压缩归档，「清理」= 保留策略；
- 同时保留 stdout 输出，docker json-file driver（max-size/max-file）作容量兜底；开发环境仅 stdout；
- 访问日志：method/path/status/duration/ip/user_id/user_role/request_id，不记请求体与 query，`/api/health*` 不记录。

### 3. 信息过滤 = 级别过滤 + 敏感字段脱敏

- `LOG_LEVEL` 全局级别过滤（生产默认 info）；
- 脱敏黑名单（password/token/secret/code/authorization/phone/email 等子串命中）字段值打码为 `***`，经日志器包装 core 对所有输出统一生效；
- 错误串内嵌连接串凭证经正则脱敏（`scheme://***@host`）。

### 4. Viper 管理配置（env-only 读取层）

- Viper `AutomaticEnv` + 顶部集中 `SetDefault` 收敛默认值；保留 `Config` 结构体、`Validate()` 生产必填校验、AI key 链式回退、CORS 自检告警语义；
- **不引入 yaml 配置文件**：生产部署是 docker env 注入（`.env` + compose），yaml 会造成配置真相双份与 dev/prod 漂移；secrets 必须走 env，yaml 只能装半套。

## 后果

- 单一日志栈、单一配置入口；排障时日志含请求 ID 与用户维度，可对单请求串起访问日志与业务日志；
- 生产日志持久化到卷，容器重建可追溯；磁盘占用有界（轮转 + docker driver 双重上限）；
- 代价：构造注入使全部 service/handler 构造签名追加参数（一次到位型改造，提交后无 slog 残留）；Viper 语义与旧 getenv 的细微差异（如非法整数回退默认值）需在测试中守护。
