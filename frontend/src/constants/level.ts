// 课程等级展示常量：中文名 → el-tag type（全局等级：入门/进阶/专项/认证）
export type LevelTagType = 'success' | 'primary' | 'warning' | 'danger' | 'info'

const LEVEL_TAG_TYPES: Record<string, LevelTagType> = {
  入门: 'success',
  进阶: 'primary',
  专项: 'warning',
  认证: 'danger'
}

export function levelTagType(name: string): LevelTagType {
  return LEVEL_TAG_TYPES[name] ?? 'info'
}
