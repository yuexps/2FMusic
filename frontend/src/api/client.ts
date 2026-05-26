import axios from 'axios'
import { getBaseUrl } from '@/utils/path'

const client = axios.create({
  baseURL: getBaseUrl(),
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 响应拦截器：若后端API返回401，说明Session过期或需重新登录，直接跳转至服务端/login页面
client.interceptors.response.use(
  (response) => {
    return response
  },
  (error) => {
    if (error.response && error.response.status === 401) {
      // 包含完整的 Hash 路径以确保刷新或登录后能准确回退到先前的二级路由子页面
      const fullPath = window.location.pathname + window.location.search + window.location.hash
      const nextPath = encodeURIComponent(fullPath)
      window.location.href = `${getBaseUrl()}/login?next=${nextPath}`
    }
    return Promise.reject(error)
  }
)

export default client
