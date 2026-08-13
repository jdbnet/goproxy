import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Login from '@/views/Login.vue'
import Dashboard from '@/views/Dashboard.vue'
import Frontends from '@/views/Frontends.vue'
import Routes from '@/views/Routes.vue'
import Backends from '@/views/Backends.vue'
import Certificates from '@/views/Certificates.vue'
import Users from '@/views/Users.vue'
import APIKeys from '@/views/APIKeys.vue'
import Audit from '@/views/Audit.vue'
import Settings from '@/views/Settings.vue'
import Help from '@/views/Help.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: Login, meta: { public: true } },
    { path: '/', name: 'dashboard', component: Dashboard },
    { path: '/frontends', name: 'frontends', component: Frontends },
    { path: '/routes', name: 'routes', component: Routes },
    { path: '/backends', name: 'backends', component: Backends },
    { path: '/certificates', name: 'certificates', component: Certificates },
    { path: '/users', name: 'users', component: Users },
    { path: '/apikeys', name: 'apikeys', component: APIKeys },
    { path: '/audit', name: 'audit', component: Audit },
    { path: '/settings', name: 'settings', component: Settings },
    { path: '/help', name: 'help', component: Help },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.checked) {
    try {
      await auth.check()
    } catch {
      auth.checked = true
    }
  }
  if (to.meta.public) {
    if (auth.authenticated && to.path === '/login') return '/'
    return true
  }
  if (auth.authRequired && !auth.authenticated) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
