import request from './request'

export const authApi = {
  login(data) {
    return request.post('/auth/login', data)
  },

  register(data) {
    return request.post('/auth/register', data)
  },

  adminLogin(data) {
    return request.post('/auth/admin-login', data)
  },

  tutorLogin(data) {
    return request.post('/auth/tutor-login', data)
  },

  logout() {
    return request.post('/auth/logout')
  },

  getUserInfo(config?) {
    return request.get('/auth/me', config)
  }
}
