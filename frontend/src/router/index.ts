import { createRouter, createWebHistory } from 'vue-router'
import HomePage from '@/pages/HomePage.vue'
import SettingsPage from '@/pages/SettingsPage.vue'
import TasksPage from '@/pages/TasksPage.vue'
import LoginPage from '@/pages/LoginPage.vue'
import AdminUsersPage from '@/pages/AdminUsersPage.vue'
import NotFoundPage from '@/pages/NotFoundPage.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomePage },
	{ path: '/tasks', name: 'tasks', component: TasksPage },
    { path: '/login', name: 'login', component: LoginPage },
    { path: '/admin/users', name: 'admin-users', component: AdminUsersPage },
    { path: '/settings', name: 'settings', component: SettingsPage },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundPage },
  ],
  scrollBehavior: () => ({ top: 0 }),
})
