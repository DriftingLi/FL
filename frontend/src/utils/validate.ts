import type { FormItemRule } from 'element-plus'

export const usernameRules: FormItemRule[] = [
  { required: true, message: '请输入用户名', trigger: 'blur' },
  { min: 3, max: 20, message: '长度在3到20个字符', trigger: 'blur' },
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

// 邮箱为选填项：仅在用户填写内容时校验格式
export const emailRules: FormItemRule[] = [
  {
    validator: (_rule, value: string, callback) => {
      if (value === '' || value == null) {
        callback()
        return
      }
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
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

// 邮箱注册/登录必填校验
export const requiredEmailRules: FormItemRule[] = [
  { required: true, message: '请输入邮箱', trigger: 'blur' },
  { pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: '请输入正确的邮箱地址', trigger: 'blur' }
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
