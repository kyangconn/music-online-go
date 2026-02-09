import { createRouter, createWebHistory } from 'vue-router'
import BaseLayout from '@/layout/BaseLayout.vue'
import Home from '@/views/Home.vue'
import Login from '@/views/auth/Login.vue'
import Register from '@/views/auth/Register.vue'
import { useUserStore } from '@/store/user'

const routes = [
  {
    path: '/',
    component: BaseLayout,
    children: [
      {
        path: '',
        name: 'Home',
        component: Home
      },
      {
        path: '/login',
        name: 'Login',
        component: Login
      },
      {
        path: '/register',
        name: 'Register',
        component: Register
      },
      // 暂时添加 placeholder 组件，防止路由报错
      {
        path: '/profile',
        name: 'Profile',
        component: () => import('@/views/user/Profile.vue').catch(() => import('@/views/Home.vue')) // 临时回退
      },
      {
        path: '/music/:id',
        name: 'MusicDetail',
        component: () => import('@/views/music/Detail.vue').catch(() => import('@/views/Home.vue'))
      },
      {
        path: '/admin',
        name: 'Admin',
        component: () => import('@/views/admin/Dashboard.vue').catch(() => import('@/views/Home.vue')),
        meta: { requiresAdmin: true }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, _from, next) => {
  const userStore = useUserStore()
  
  if (to.meta.requiresAuth && !userStore.isLoggedIn) {
    next('/login')
  } else if (to.meta.requiresAdmin && !userStore.isAdmin) {
    next('/')
  } else {
    next()
  }
})

export default router
