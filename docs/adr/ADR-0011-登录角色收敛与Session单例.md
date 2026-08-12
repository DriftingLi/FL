# ADR-0011: 登录角色收敛与 Session 单例（含 admin 无禁用语义的负向决策）

- 状态：已接受（2026-08-12，架构深化第三波 spec #148）
- 领域：账号与认证 —— 登录 / 会话（session）

## 背景

HrwaiLogin / AdminLogin / TutorLogin 三个 ~25 行函数同构（查表 → 验密 → 禁用校验 → 签发 → 组结果），差异只有表、角色名与错误文案——复制形态下任何一处口径调整都可能在其余两处漂移。会话实例存在三份：AuthService 内部自建（且 CookieConfig 为空，与配置派生的实例存在属性漂移风险）、Deps.Session、估值模块装配——签发（AuthService 内部实例）与校验（Deps 实例）靠「同一配置巧合一致」工作；AuthService.Session() 死委托零调用，ExtractToken/RevokeToken 窄接口补丁仅为估值模块的 Logout 服务。

另需记录一个事实核查结果：admin 表无 status 字段（admin_id/username/password/name/created_at），管理端状态开关端点仅存在于 hrwai 用户与讲师——「AdminLogin 缺禁用校验」不是 bug。

## 决策

### 1. Session 装配根单例

- 会话实例在 NewDeps 创建一次，注入 AuthService（构造参数）/ Deps.Session / 估值模块路由（main.go 经 deps.Session 传递）；
- AuthService.Session() 死委托与 ExtractToken / RevokeToken 窄接口补丁删除，估值 Logout 直接用注入的会话实例提取与吊销 token。

### 2. 登录骨架收敛

- 验密 → 禁用校验 → 签发 → 结果组装收敛为共享骨架（verifyAndIssue），三个登录入口只保留各自查表与错误文案差异；
- 角色差异（是否校验 status）声明式表达：hrwai 用户与讲师校验 status=1，admin 不校验（表无该字段）。

### 3. 负向决策：不引入 admin 禁用语义

- 不新增 admin.status 字段与对应管理端点；若产品需要管理员禁用，另行立项（DB 迁移 + 管理面 + 登录校验三处联动）。

## 约束

- 登录/登出/黑名单行为与既有完全一致（含错误文案）；JWT claims 结构不变（ADR-0002）。

## 后果

- 签发与校验同实例，配置漂移即全员登出的隐患消除；
- 登录口径（禁用校验、错误文案）只维护一份；
- 未来架构评审不会再把「AdminLogin 缺禁用校验」当作 bug 提交（本 ADR 为锚点）。

## 相关

- `backend/internal/service/auth_service.go`、`internal/api/deps.go`、`internal/valuation/handler/auth.go`
- ADR-0002（会话单一接口——本决策兑现其「中间件与 AuthService 各自持有实例」的代价项）
