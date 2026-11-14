/**
 * 章节文件与审核状态的**词表单点**（ui-conventions「状态/类型词表：单一 descriptor 派生 label 与 tone」）。
 *
 * 三处读面共用这一份，避免各自 Record 漂移：
 * - 管理端审核队列（快照文件清单 + 审核状态标签）
 * - 讲师端文件库（类型标签）
 * - 讲师端章节编辑（挂接来源）
 *
 * 取值集合来自后端 `backend/internal/filestore/file_type_table.go` 的 upload 四档
 * （document / ppt / video / image）与 `course.ChapterFileDTO.source` 三值
 * （chapter_file / tutor_file / legacy）。
 */
import type { IconName } from '@/icons/registry'

export type DisplayTone = 'brand' | 'success' | 'info' | 'neutral' | 'warning' | 'danger'

export interface DisplayDescriptor {
  label: string
  tone: DisplayTone
}

const CATEGORY: Record<string, DisplayDescriptor> = {
  video: { label: '视频', tone: 'brand' },
  document: { label: '文档', tone: 'info' },
  ppt: { label: '幻灯片', tone: 'success' },
  image: { label: '图片', tone: 'neutral' }
}

/** 文件类型 → 中文标签与色调。未知类型原样显示（后端新增档位时不会变成空白）。 */
export function describeFileCategory(contentType: string): DisplayDescriptor {
  return CATEGORY[contentType] ?? { label: contentType || '未知类型', tone: 'neutral' }
}

const CATEGORY_ICON: Record<string, IconName> = {
  video: 'video',
  ppt: 'monitor',
  image: 'image',
  document: 'file-text'
}

/** 文件类型 → 图标名（与 describeFileCategory 同源，避免第二份映射漂移）。 */
export function fileCategoryIcon(contentType: string): IconName {
  return CATEGORY_ICON[contentType] ?? 'file-text'
}

const SOURCE: Record<string, DisplayDescriptor> = {
  chapter_file: { label: '章节附件', tone: 'neutral' },
  tutor_file: { label: '文件库挂接', tone: 'brand' },
  legacy: { label: '历史附件', tone: 'neutral' }
}

/** 文件来源 → 中文标签（后端枚举不得原样印给用户）。 */
export function describeFileSource(source?: string): DisplayDescriptor {
  if (!source) return SOURCE.chapter_file
  return SOURCE[source] ?? { label: source, tone: 'neutral' }
}

/** 章节审核状态 → descriptor（C1 三态；draft 同时承载「草稿」与「被驳回待改」两种人读语义）。 */
export function describeReviewStatus(status?: string): DisplayDescriptor {
  switch (status) {
    case 'pending':
      return { label: '审核中', tone: 'info' }
    case 'published':
      return { label: '已发布', tone: 'success' }
    case 'draft':
      return { label: '草稿', tone: 'warning' }
    default:
      return { label: '草稿', tone: 'warning' }
  }
}
