import axios from "axios"
import { ElMessage } from "element-plus"

const service = axios.create({
  baseURL: "/api/v1",
  timeout: 5000,
})

service.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem("token")
    if (token) {
      config.headers["Authorization"] = "Bearer " + token
    }
    return config
  },
  (error) => Promise.reject(error),
)

service.interceptors.response.use(
  (response) => {
    const res = response.data
    return res
  },
  (error) => {
    if (error.response) {
      const status = error.response.status
      const data = error.response.data

      if (status === 401) {
        localStorage.removeItem("token")
        localStorage.removeItem("user")

        if (data?.error?.includes("expired")) {
          ElMessage.warning("Session expired, please login again")
        } else {
          ElMessage.warning("Please login to continue")
        }

        const currentPath = window.location.pathname
        if (currentPath !== "/login" && currentPath !== "/register") {
          window.location.href = "/login"
        }
      } else if (status === 403) {
        console.warn("Forbidden:", data)
      } else if (status >= 500) {
        console.error("Server error:", status, data)
      }
    } else if (error.request) {
      console.error("Network error")
    }

    return Promise.reject(error)
  },
)

export default service
