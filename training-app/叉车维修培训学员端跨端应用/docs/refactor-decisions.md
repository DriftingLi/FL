# 重构决策清单（spec #638 页面手术结论）

> 2026-09-08 定稿 · 依据 T02 mall（#640）与 T03 profile（#641 / #675–#678 / #684）手术结论收束；epic 总纲见 #638，playbook 见 ADR-0007。

1. 渐进绞杀：每模块 1 分支 + 1 PR + 1 专注会话；合并即直发 production，回滚 = revert squash commit。
2. 顶层骨架不动；巨型页面拆 section 组件/composable，放 `pages/<module>/components|composables/` 显式 import，不进 easycom 全局目录。
3. 单文件软预算 ≤600 行已由 mall/profile 两模块验证成立：profile 四文件 1011→592、722→448、1137→565、687→503，全模块 19 文件复检达标，无过度碎片化。
4. 纯展示/纯动作段优先拆，成本低收益高；业务数据区（列表+分页等）留主文件不硬拆。
5. UI 像素级冻结：冒烟基线 = 前后逐页截图对比；手术期间发现问题另立 issue（如 #700 后端缺读接口），不顺手改。
6. api 收紧统一走 mapper-callback 出口家族（requestMapped/getMapped/postMapped）；既有 request/get/post 原样保留（expand–contract）。
7. 安全网 = 契约测试机检（预算/接线/数据所有权/allowlist 防回潮）+ 静态守护 + 四门冒烟；不新增组件级测试。
8. **jest 全绿 ≠ 能编译**：Kotlin 传参/模板形态盲区只有 HBuilderX 全量编译门能拦，每手术 PR 必过（error17/18 五层实证）。
9. 拆分时数据所有权必须单一：谁消费谁持有，跨组件要数据扁平下发；禁止 `ref<any>`/`defineExpose` 反向桥接（守护规则 Q）。
10. 可选对象类型 prop 成员直读在 Kotlin 必炸：对象 prop 扁平化为原始 props 或走工厂默认值（规则 N/P）。
11. 机械禁列已落守护 M–S：裸 builder 引用、`as unknown as`、模板裸插值 function、`ref<any`、`async : void`（须 `: Promise<void>`）、模板直调 import 函数（本地薄包装）。
12. 展示纯函数收敛 utils，消除页面/组件双实现（wrongQuestionDisplay、formatDateStr 先例）。
13. 契约测试迁移纪律：同 PR 可改路径指向，禁删/弱化断言；既有欠账 #657（2 红）可指认即可、另行处理。
14. 600 预算是否硬化：forum（T04）为第三个复评模块，术后按 User Story 22 决断。
15. 后续照 spec 序列推进：forum → dashboard → practice → exam → courses → resume → ai-assistant → auth 三兄弟；login pin 在生物识别后；allowlist 由各手术票清自己范围、#654 收尾删机制。
