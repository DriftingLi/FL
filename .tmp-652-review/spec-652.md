## Parent

#638（refactor epic，T14/16，api 收尾批次 A）

## What to build

非手术模块的域 api 收紧第一批：points、jobs、featured、search、guide、notifications 六个域的 api 文件全部改为显式 DTO + 手动映射，经 T01 mapper-callback 出口（#639）。**纯 api 层改动，页面零改动**——这六个模块的页面不在手术清单，靠本批继承契约收紧。

## Acceptance criteria

- [ ] 六个域 api 全部导出显式 DTO 类型 + 手动映射函数，经 mapper-callback 出口
- [ ] 调用方页面仅机械适配 import/类型名（零行为改动）
- [ ] api/ 层 allowlist 条目（catch/detail）清零
- [ ] jest 全绿（npm test / test:unit，含各域既有契约测试）
- [ ] HBuilderX 真机运行全量编译无新增 error（人工）
- [ ] 分支 refactor/api-batch-a，PR 关联本 issue；合并即直发 production

## Blocked by

- #640（T02 mall 试点 + ADR-0007 草案，收紧约定以 ADR-0007 为准）
