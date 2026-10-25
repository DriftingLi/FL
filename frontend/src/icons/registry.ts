import { Check, ChevronDown, CircleAlert, LogOut, Menu, Moon, Sun, SunMoon, UserRound } from 'lucide'
import type { IconNode } from './types'

/**
 * 图标注册表：语义名 → 24 网格图形数据。全站唯一的数据源（ADR-0079）。
 *
 * - key 是 kebab-case 的**语义名**，不是上游库的 PascalCase 名：换数据源时调用方零改动；
 * - 新增图标只改这张表。`IconName` 由它推断，写错名字在 `npm run type-check` 阶段就红；
 * - 本票（#1911）登记主题三态 + 顶栏样板（9 个）与降级图标；其余 60 来个名字随 #1912 的
 *   全量迁移一起进来（实心图标的数据也在那一票抽取）。
 */
export const ICONS = {
  /** 浅色主题 */
  sun: Sun,
  /** 深色主题 */
  moon: Moon,
  /**
   * 跟随系统：半日半月。
   *
   * 原先是 ThemeToggle 里手绘的四段 SVG（两段填色 + 两段描边，描边 1.5），迁移时换成
   * 描边口径的半日半月：三态要「视觉重量一致」，而形变驱动把整个图标合成**一条 path**，
   * 混合填充/描边的图形撑不过那一刻（见 ADR-0079）。
   */
  'theme-system': SunMoon,
  /** 选中/已完成 */
  check: Check,
  /** 折叠箭头 / 下拉指示 */
  'chevron-down': ChevronDown,
  /** 汉堡菜单 */
  menu: Menu,
  /** 用户身份 */
  user: UserRound,
  /** 退出登录 */
  'log-out': LogOut,
  /** 未登记名字的降级图形 */
  'circle-alert': CircleAlert
} as const satisfies Record<string, IconNode>

/** 注册表里的全部语义名。 */
export type IconName = keyof typeof ICONS

/** 未登记名字的降级目标：看得见的警示图形，不白屏。 */
export const FALLBACK_ICON: IconName = 'circle-alert'

/** 名字是否已登记（运行期判据，类型面由 `IconName` 负责）。 */
export function hasIcon(name: string): name is IconName {
  return Object.prototype.hasOwnProperty.call(ICONS, name)
}

/** 取图形；未登记时回落，调用方拿到的永远是可渲染的数据。 */
export function resolveIcon(name: IconName): { node: IconNode; fallback: boolean } {
  if (hasIcon(name)) return { node: ICONS[name], fallback: false }
  return { node: ICONS[FALLBACK_ICON], fallback: true }
}
