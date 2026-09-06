// vue-router RouteMeta 扩展（#618）：路由 meta 增加单一工作区声明，与既有 role/roles 并列。
// 工作区语义（training/tutor/valuation/recruit/manage/auth）而非子域名字面——
// 「工作区」与「子域名」是两个概念（IP 直连部署形态下子域概念不存在），
// 工作区 → 子域名的派生单点在 router/guard.ts 的 WORKSPACE_SUBDOMAIN。
import 'vue-router'
import type { Workspace } from '@/router/guard'

declare module 'vue-router' {
  // eslint-disable-next-line @typescript-eslint/no-empty-object-type
  export interface RouteMeta {
    /** 工作区声明：守卫与登录回跳读声明而非猜路径前缀（未声明的未匹配路径走 404 前缀表兜底） */
    workspace?: Workspace
  }
}
