import { createRouter, createWebHistory } from 'vue-router'
import HomePage from '@/pages/HomePage.vue'
import TasksPage from '@/pages/TasksPage.vue'
import LoginPage from '@/pages/LoginPage.vue'
import SetupPage from '@/pages/SetupPage.vue'
import RegisterPage from '@/pages/RegisterPage.vue'
import AccountPage from '@/pages/AccountPage.vue'
import AdminUsersPage from '@/pages/AdminUsersPage.vue'
import NotFoundPage from '@/pages/NotFoundPage.vue'
import { useAuthStore } from '@/stores/auth'
import { resolvePostLoginPath } from '@/utils/postLogin'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomePage },
    { path: '/setup', name: 'setup', component: SetupPage },
    { path: '/login', name: 'login', component: LoginPage },
    { path: '/register', name: 'register', component: RegisterPage },
    { path: '/tasks', name: 'tasks', component: TasksPage, meta: { requiresAuth: true } },
    { path: '/admin/users', name: 'admin-users', component: AdminUsersPage, meta: { requiresAdmin: true } },
    { path: '/account', name: 'account', component: AccountPage, meta: { requiresAuth: true } },
    { path: '/settings', redirect: '/account' },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundPage },
  ],
  scrollBehavior: () => ({ top: 0 }),
})
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) {
    try { await auth.initialize() } catch { if (to.name !== 'setup') return { name: 'setup', query: { unavailable: '1' } } }
  }
  if (auth.setupRequired && to.name !== 'setup') return { name: 'setup' }
  if (!auth.setupRequired && to.name === 'setup') return { name: auth.authenticated ? 'home' : 'login' }
  if ((to.meta.requiresAuth || to.meta.requiresAdmin) && !auth.authenticated) {
    const returnTo = resolvePostLoginPath(to.fullPath)
    return returnTo === '/' ? { name: 'login' } : { name: 'login', query: { returnTo } }
  }
  if (to.meta.requiresAdmin && !auth.isAdmin) return { name: 'home' }
  if ((to.name === 'login' || to.name === 'register') && auth.authenticated) return { name: 'home' }
})
