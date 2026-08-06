// 公共常量
// 重构说明：删除旧 ITEM_STATUS_OPTIONS / WORK_CONDITION_OPTIONS / FUEL_TYPE_OPTIONS / BRAND_TIER_LABEL
//         字典选项统一从后端动态加载，前端只保留车况评级展示色
//         系数定义与维度标签由 API 数据驱动（dimension_scores 返回 label），前端不再维护矛盾定义

/** 车况评级展示色（A 绿 → E 红） */
export const CONDITION_RATING_COLOR: Record<string, string> = {
  A: '#16A34A',
  B: '#0EA5E9',
  C: '#F59E0B',
  D: '#F97316',
  E: '#DC2626'
}
