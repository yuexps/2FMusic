import axios from 'axios'
import { getBaseUrl } from '@/utils/path'

const client = axios.create({
  baseURL: getBaseUrl(),
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 自动注入本地密码凭证
client.interceptors.request.use((config) => {
  const pass = localStorage.getItem('2fmusic_password')
  if (pass) {
    config.headers['X-Password'] = pass
  }
  return config
})

// 401 自动拉起全屏登录解锁弹窗
client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 401) {
      window.dispatchEvent(new CustomEvent('2fmusic-unauthorized'))
    }
    return Promise.reject(error)
  }
)

export default client
