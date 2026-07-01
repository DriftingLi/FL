export const usernameRules = [
  { required: true, message: '请输入用户名', trigger: 'blur' },
  { min: 3, max: 20, message: '长度在3到20个字符', trigger: 'blur' },
  { pattern: /^[a-zA-Z0-9_]+$/, message: '只能包含字母、数字和下划线', trigger: 'blur' }
]

export const passwordRules = [
  { required: true, message: '请输入密码', trigger: 'blur' },
  { min: 6, max: 20, message: '长度在6到20个字符', trigger: 'blur' }
]

export const nameRules = [
  { required: true, message: '请输入姓名', trigger: 'blur' },
  { min: 2, max: 10, message: '长度在2到10个字符', trigger: 'blur' }
]

export const phoneRules = [
  { required: true, message: '请输入手机号', trigger: 'blur' },
  { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的11位手机号', trigger: 'blur' }
]

// 邮箱为选填项：仅在用户填写内容时校验格式
export const emailRules = [
  {
    validator: (rule, value, callback) => {
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
export const companyRules = [
  { max: 50, message: '长度不能超过50个字符', trigger: 'blur' }
]

export const confirmPasswordRule = (formRef, fieldName = 'password') => ({
  validator: (rule, value, callback) => {
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
