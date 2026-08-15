import type { FormItemRule } from 'element-plus'

// ===== 格式校验单一事实源（useSendCode 与表单规则共用） =====

// 邮箱格式：local@domain，domain 含点（与后端 IsValidEmail 的 net/mail + 域名含点语义对齐，
// 拒绝 a@b 这类无点域）。isValidEmail 与 requiredEmailRules 共用这一单一模式（seam 单一事实源）。
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/** 邮箱：与后端 net/mail + 域名含点语义对齐（拒绝无点域 a@b，接受 a@b.c）。 */
export const isValidEmail = (value: string): boolean => EMAIL_PATTERN.test(value)

/** 手机号：11 位，1[3-9] 开头 */
export const isValidPhone = (value: string): boolean => /^1[3-9]\d{9}$/.test(value)

/** 登录账号：与后端 IsValidAccount 对齐（4-20 位字母/数字/下划线） */
export const isValidAccount = (value: string): boolean => /^[A-Za-z0-9_]{4,20}$/.test(value)

export const usernameRules: FormItemRule[] = [
  { required: true, message: '请输入账号', trigger: 'blur' },
  { min: 4, max: 20, message: '长度在4到20个字符', trigger: 'blur' },
  { pattern: /^[a-zA-Z0-9_]+$/, message: '只能包含字母、数字和下划线', trigger: 'blur' }
]

export const passwordRules: FormItemRule[] = [
  { required: true, message: '请输入密码', trigger: 'blur' },
  { min: 6, max: 20, message: '长度在6到20个字符', trigger: 'blur' }
]

export const nameRules: FormItemRule[] = [
  { required: true, message: '请输入姓名', trigger: 'blur' },
  { min: 2, max: 10, message: '长度在2到10个字符', trigger: 'blur' }
]

// 注册昵称（1-30 字符）
export const nicknameRules: FormItemRule[] = [
  { required: true, message: '请输入昵称', trigger: 'blur' },
  { min: 1, max: 30, message: '昵称长度需在1到30个字符', trigger: 'blur' }
]

export const phoneRules: FormItemRule[] = [
  { required: true, message: '请输入手机号', trigger: 'blur' },
  { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的11位手机号', trigger: 'blur' }
]

// 邮箱为选填项：仅在用户填写内容时校验格式（与后端 net/mail 语义对齐）
export const emailRules: FormItemRule[] = [
  {
    validator: (_rule, value: string, callback) => {
      if (value === '' || value == null) {
        callback()
        return
      }
      if (!isValidEmail(value)) {
        callback(new Error('请输入正确的邮箱地址'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }
]

// 单位为选填项：仅校验长度上限
export const companyRules: FormItemRule[] = [
  { max: 50, message: '长度不能超过50个字符', trigger: 'blur' }
]

// 邮箱注册/登录必填校验（与后端 net/mail 语义对齐）
export const requiredEmailRules: FormItemRule[] = [
  { required: true, message: '请输入邮箱', trigger: 'blur' },
  { pattern: EMAIL_PATTERN, message: '请输入正确的邮箱地址', trigger: 'blur' }
]

export const emailCodeRules: FormItemRule[] = [
  { required: true, message: '请输入验证码', trigger: 'blur' },
  { pattern: /^\d{6}$/, message: '验证码为6位数字', trigger: 'blur' }
]

type FormModel = Record<string, unknown>

export const confirmPasswordRule = (formRef: FormModel, fieldName = 'password'): FormItemRule => ({
  validator: (_rule, value: string, callback) => {
    if (value === '') {
      callback(new Error('请再次输入密码'))
    } else if (value !== formRef[fieldName]) {
      callback(new Error('两次输入密码不一致'))
    } else {
      callback()
    }
  },
  trigger: 'blur'
})
