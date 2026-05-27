import axios from 'axios'
import { getBaseUrl } from '@/utils/path'

const client = axios.create({
  baseURL: getBaseUrl(),
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 401 时跳转登录页面
client.interceptors.response.use(
  (response) => {
    return response
  },
  (error) => {
    if (error.response && error.response.status === 401) {
      // 携带完整路径便于登录后回退
      const fullPath = window.location.pathname + window.location.search + window.location.hash
      const nextPath = encodeURIComponent(fullPath)
      window.location.href = `${getBaseUrl()}/login?next=${nextPath}`
    }
    return Promise.reject(error)
  }
)

export default client
