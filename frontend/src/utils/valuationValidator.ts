// 前端参数校验工具
// 与后端 model/evaluation.go 中的校验逻辑保持一致

const CURRENT_YEAR = new Date().getFullYear()

export interface ValidationResult {
  valid: boolean
  message?: string
}

/** 校验品牌：非空 */
export function validateBrand(brand: string | undefined | null): ValidationResult {
  if (!brand || !brand.trim()) {
    return { valid: false, message: '请选择品牌' }
  }
  return { valid: true }
}

/** 校验价格：> 0 且 ≤ 9999 */
export function validateOriginalPrice(price: number | undefined | null): ValidationResult {
  if (price == null || Number.isNaN(price)) {
    return { valid: false, message: '请输入原始价格' }
  }
  if (price <= 0) {
    return { valid: false, message: '价格必须大于 0' }
  }
  if (price > 9999) {
    return { valid: false, message: '价格超出合理范围' }
  }
  return { valid: true }
}

/** 校验年份：购置/成交年份合法性 */
export function validateYears(purchase: number | undefined, sale: number | undefined): ValidationResult {
  if (purchase == null || sale == null) {
    return { valid: false, message: '请填写购置与成交年份' }
  }
  if (!Number.isInteger(purchase) || !Number.isInteger(sale)) {
    return { valid: false, message: '年份必须为整数' }
  }
  if (purchase < 1980 || purchase > CURRENT_YEAR) {
    return { valid: false, message: `购置年份应在 1980~${CURRENT_YEAR} 之间` }
  }
  if (sale < purchase) {
    return { valid: false, message: '成交年份不能早于购置年份' }
  }
  if (sale > CURRENT_YEAR + 1) {
    return { valid: false, message: `成交年份不合法（>${CURRENT_YEAR + 1}）` }
  }
  return { valid: true }
}

/** 校验使用小时：≥ 0 且 ≤ 100000 */
export function validateUsageHours(hours: number | undefined | null): ValidationResult {
  if (hours == null || Number.isNaN(hours)) {
    return { valid: false, message: '请填写累计使用小时' }
  }
  if (hours < 0) {
    return { valid: false, message: '使用小时不能为负数' }
  }
  if (hours > 100000) {
    return { valid: false, message: '使用小时超出合理范围' }
  }
  return { valid: true }
}

/** 校验工况 */
export function validateWorkCondition(condition: string | undefined | null): ValidationResult {
  const valid = ['仓储', '港口', '冷库', '工地', '其他']
  if (!condition || !valid.includes(condition)) {
    return { valid: false, message: '请选择使用工况' }
  }
  return { valid: true }
}

/** 校验部件状态：是否所有类别下的条目都有状态 */
export function validateAllItemsAssigned(
  itemCodes: string[],
  statusMap: Record<string, string | undefined>
): ValidationResult {
  if (itemCodes.length === 0) {
    return { valid: false, message: '未配置部件条目' }
  }
  const missing = itemCodes.filter((c) => !statusMap[c])
  if (missing.length > 0) {
    return { valid: false, message: `还有 ${missing.length} 个部件未评估` }
  }
  return { valid: true }
}
