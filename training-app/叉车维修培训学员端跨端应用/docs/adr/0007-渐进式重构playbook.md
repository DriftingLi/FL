# 0007 - 渐进式重构 playbook：页面层手术 + api 契约收紧

**状态**：草案（T02 mall 试点产出，待试点复盘通过后定稿）
**来源**：重构 epic #638（四轮 grilling 收束），试点工单 #640

## 背景与目标

学员端（uni-app-x，约 4.2 万行、53 页）随迭代出现巨型文件（20 个源文件超 600 行）、页面层重复无沉淀、api 契约不一致（请求基座裸 `any`、部分域无 DTO）。目标：治类型/编译痛点 + 可维护性 + 为功能迭代提速。**性能与 UI 重设计不在目标内。**

## 决策

### 形态：渐进绞杀，非大爆炸

- 每模块 1 分支（`refactor/<module>`）+ 1 PR + 1 专注会话；每 PR 合并即直发 production（squash revert 即回滚），不改 CD 流水线
- 模块级冻结：手术期间该模块冻结新功能；**纯机械 import 迁移不算破坏冻结**
- 手术顺序按体量降序（mall 试点先行）；login 单独 pin 在生物识别线合并后

### 靶子：页面层手术，骨架不动

- `pages/ components/ api/ stores/ utils/` 顶层保留，不做 `features/` 迁移（pages.json 53 路径 churn + easycom 约束 + UTS import 脆弱，负收益）
- 巨型页面拆 section 组件/composable；不拆子路由（除非产品本就是多页）
- **单文件软预算 ≤600 行**（全文件计），模块目录 ≤2 层。试点起以契约测试机检锁定已达标模块（防回潮），未达标模块不强制全库扫描
- **UI 像素级冻结**：界面必须与手术前一致，UI 问题记 issue 不顺手改。冒烟基线 = 前后逐页截图对比

### api 契约收紧：requestMapped / getMapped 出口家族

- 出口形态 = mapper-callback（与 ADR-0003 手动 JSON 映射同构）：`requestMapped<T>(opts, map)`；GET 走便捷面 `getMapped<T>(url, params, map)`——**params 沿用手动 query 序列化，不交平台自动序列化**（试点发现：跨端序列化行为有漂移风险，`get()` 的手动拼接是实证安全的行为，收紧不得改变请求形态）
- 每域 api 导出显式 DTO + build* 映射函数；页面层只 import 域 api（2026-09 探针实测页面层直接 `uni.request` 为 0，暂无独立守护规则；若新增违例在 #654 收尾票立项）
- 既有 `request/get/post` 原样保留（expand–contract），页面接线迁移按票推进

### 拆出物安置与晋升

- 模块私有组件/composable 放 `pages/<module>/components|composables/`，**显式 import**（PascalCase 标签 + kebab-case 事件绑定，同 ai-chat 先例；easycom 仅全局件）
- 全局 `components/`、`composables/` 只放已确认跨模块复用件；**第二个模块需要时晋升全局**，晋升 PR 允许机械改使用方 import
- 手术 PR 内允许增量式抽取（新建共享件 + 当前模块采用），不碰其他模块行为代码

### 静态守护与契约测试纪律

- `utsAndroidCompile.test.js` 沿用「纯函数扫描 + 注入违规自检 + 全工程断言」模式按遭遇增量扩规则；存量违例走 `GUARD_ALLOWLIST`（expand 侧），各手术票清自己范围，epic 收尾票删机制
- 手术 PR 内契约测试可更新文件路径指向，**禁删/弱化断言**（弱化须引用 issue）
- 不新增组件级测试（uvue 无生态）；行为回归靠四门冒烟

### 每手术 PR 四门验收

1. Android 真机逐页截图前后对比（人工签收）
2. 微信开发者工具逐页打开无报错（人工）
3. `npm run test:unit` 全绿（既有失败须可指认到独立 issue）
4. HBuilderX 真机运行全量编译无新增 error（人工）

**push 前先 `git merge origin/master`**（试点教训：分支基点过期会同时触发 ruleset up-to-date 门禁与迁移版本预检两道拦截）。

## 试点复盘（keep–adjust）

- **keep**：拆纯展示/纯动作段（排序栏、悬浮组）成本低收益高；业务数据区（商品列表+必买清单+分页加载）留主文件不硬拆；600 预算 442 行达标且未过度碎片化
- **keep**：契约测试机检组件接线（防"import 了但没用"与静默回退）
- **adjust**：出口家族需 GET 便捷面（getMapped）——spec 只写了 requestMapped，试点第一步即暴露 GET 参数序列化的形态保持问题
- **教训**：搬迁代码前逐符号核对消费面（试点中 mustBuyList 因与悬浮按钮相邻被误删，幸被契约测试前的人工核查拦下）

## 关联

- 工单序列：#639（T01 基建）→ #640（T02 本试点）→ #641–#653（T03–T13 手术）→ #652/#653（api 批量收尾）→ #654（allowlist 退役）
- ADR-0002（轻量状态管理）、ADR-0003（手动 JSON 映射）维持；本 ADR 是其执行细则
