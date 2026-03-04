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
    console.log('err' + error) // for debug
    return Promise.reject(error)
  }
)

export default service
