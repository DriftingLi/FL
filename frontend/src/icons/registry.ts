import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ArrowUpDown,
  Bell,
  Bookmark,
  BookOpen,
  Building2,
  BriefcaseMedical,
  Calendar,
  Camera,
  ChartLine,
  ChartNoAxesColumn,
  ChartPie,
  Check,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  CircleAlert,
  CircleCheck,
  CircleQuestionMark,
  CircleX,
  ClipboardList,
  Clock,
  Copy,
  Download,
  Ellipsis,
  Eye,
  FileText,
  Files,
  Flag,
  FolderOpen,
  Funnel,
  GraduationCap,
  Image,
  IdCard,
  Info,
  List,
  ListChecks,
  LoaderCircle,
  Lock,
  LogOut,
  Mail,
  Maximize,
  Menu,
  MessageCircle,
  MessageSquare,
  Monitor,
  Moon,
  Notebook,
  Paperclip,
  PanelLeftClose,
  PanelLeftOpen,
  Pause,
  Pencil,
  Phone,
  Play,
  Plus,
  RotateCcw,
  RotateCw,
  Search,
  Send,
  Settings,
  Settings2,
  Shield,
  Sparkles,
  SquarePen,
  Star,
  StickyNote,
  Sun,
  SunMoon,
  Tag,
  Timer,
  Trash2,
  TriangleAlert,
  Trophy,
  Upload,
  UploadCloud,
  UserRound,
  Video,
  WandSparkles,
  Wrench,
  X,
  ZoomIn,
  ZoomOut
} from 'lucide'
import { FILLED_ICONS } from './filled'
import type { IconNode } from './types'

/**
 * 图标注册表：语义名 → 24 网格图形数据。全站唯一的数据源（ADR-0079）。
 *
 * - key 是 kebab-case 的**语义名**，不是上游库的 PascalCase 名：换数据源时调用方零改动；
 * - 新增图标只改这张表。`IconName` 由它推断，写错名字在 `npm run type-check` 阶段就红；
 * - 实心图标（`*-filled`）在 `filled.ts`：生成物，从 Element Plus 图标重网格而来，勿手改。
 */
export const ICONS = {
  ...FILLED_ICONS,

  // ── 主题三态 ─────────────────────────────────────────────
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

  // ── 方向与排序 ───────────────────────────────────────────
  'arrow-right': ArrowRight,
  'arrow-left': ArrowLeft,
  'arrow-down': ArrowDown,
  'arrow-up': ArrowUp,
  'arrow-up-down': ArrowUpDown,
  'chevron-down': ChevronDown,
  'chevron-up': ChevronUp,
  'chevron-right': ChevronRight,

  // ── 通用操作 ─────────────────────────────────────────────
  plus: Plus,
  x: X,
  check: Check,
  search: Search,
  'loader-circle': LoaderCircle,
  'rotate-cw': RotateCw,
  'rotate-ccw': RotateCcw,
  eye: Eye,
  pencil: Pencil,
  'square-pen': SquarePen,
  'trash-2': Trash2,
  ellipsis: Ellipsis,
  copy: Copy,
  send: Send,

  // ── 表单、筛选与时间 ─────────────────────────────────────
  funnel: Funnel,
  calendar: Calendar,
  timer: Timer,
  clock: Clock,

  // ── 提示与状态 ───────────────────────────────────────────
  bell: Bell,
  flag: Flag,
  info: Info,
  'triangle-alert': TriangleAlert,
  'circle-check': CircleCheck,
  'circle-x': CircleX,
  'circle-question': CircleQuestionMark,
  star: Star,
  /** 未登记名字的降级图形 */
  'circle-alert': CircleAlert,
  /** 密码 / 私密 */
  lock: Lock,
  /** 邮箱（表单前缀） */
  mail: Mail,
  /** 证件号（表单前缀） */
  'id-card': IdCard,
  /** 单位 / 公司（表单前缀） */
  'building-2': Building2,

  // ── 媒体与文件 ───────────────────────────────────────────
  image: Image,
  video: Video,
  play: Play,
  pause: Pause,
  maximize: Maximize,
  'zoom-in': ZoomIn,
  'zoom-out': ZoomOut,
  camera: Camera,
  download: Download,
  upload: Upload,
  'upload-cloud': UploadCloud,

  // ── 导航、身份与内容 ─────────────────────────────────────
  menu: Menu,
  'log-out': LogOut,
  'panel-left-close': PanelLeftClose,
  'panel-left-open': PanelLeftOpen,
  user: UserRound,
  notebook: Notebook,
  'file-text': FileText,
  files: Files,
  'folder-open': FolderOpen,
  list: List,
  'list-checks': ListChecks,
  'clipboard-list': ClipboardList,
  bookmark: Bookmark,
  tag: Tag,
  'sticky-note': StickyNote,
  trophy: Trophy,
  'chart-pie': ChartPie,
  'chart-line': ChartLine,
  'chart-no-axes-column': ChartNoAxesColumn,
  wrench: Wrench,
  settings: Settings,
  'settings-2': Settings2,
  shield: Shield,
  sparkles: Sparkles,
  'graduation-cap': GraduationCap,
  'book-open': BookOpen,
  'briefcase-medical': BriefcaseMedical,
  'message-square': MessageSquare,
  paperclip: Paperclip,
  'message-circle': MessageCircle,
  phone: Phone,
  'wand-sparkles': WandSparkles,
  monitor: Monitor
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
