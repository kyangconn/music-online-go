import axios from 'axios'

// 创建 axios 实例
const service = axios.create({
  // URL 前缀，因为在 vite.config.ts 中配置了代理，所以这里直接写 /api/v1 即可
  // 生产环境(Go embed)下，也是同样的路径
  baseURL: '/api/v1', 
  timeout: 5000 // 请求超时时间
})

// 请求拦截器
service.interceptors.request.use(
  config => {
    // 在这里可以统一添加 token，例如：
    const token = localStorage.getItem('token')
    if (token) {
      config.headers['Authorization'] = 'Bearer ' + token
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

// 响应拦截器
service.interceptors.response.use(
  response => {
    const res = response.data
    // 这里可以根据后端的通用响应结构做统一处理
    // 比如假设后端返回 { code: 0, data: ..., msg: ... }
    // if (res.code !== 0) {
    //   // 处理错误
    //   return Promise.reject(new Error(res.msg || 'Error'))
    // }
    return res
  },
  error => {
    // 统一错误处理
    console.error('API请求错误:', error)
    
    // 可以根据不同的错误类型进行不同的处理
    if (error.response) {
      // 服务器返回了错误状态码
      console.error('错误状态码:', error.response.status)
      console.error('错误数据:', error.response.data)
      
      // 处理特定的HTTP状态码
      if (error.response.status === 401) {
        // 未授权，可能需要跳转到登录页面
        console.warn('用户未授权，可能需要重新登录')
      } else if (error.response.status === 403) {
        // 禁止访问
        console.warn('访问被拒绝，权限不足')
      } else if (error.response.status === 404) {
        // 资源未找到
        console.warn('请求的资源不存在')
      } else if (error.response.status >= 500) {
        // 服务器错误
        console.error('服务器内部错误')
      }
    } else if (error.request) {
      // 请求已发出但没有收到响应
      console.error('网络错误，无法连接到服务器')
    } else {
      // 请求配置错误
      console.error('请求配置错误:', error.message)
    }
    
    return Promise.reject(error)
  }
)

export default service
