import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardView },
    { path: '/users', name: 'users', component: () => import('../views/UsersView.vue') },
    { path: '/admins', name: 'admins', component: () => import('../views/AdminsView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

