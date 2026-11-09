<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { logout } from '../services/http'
import { useSession } from '../stores/session'

const route = useRoute()
const session = useSession()
const mobileOpen = ref(false)
const initials = computed(() => (session.state.profile?.username || 'A').slice(0, 1).toUpperCase())
const roleName = computed(() => session.state.profile?.role === 1 ? '超级管理员' : '管理员')
const links = [
  { to: '/', label: '概览', icon: '⌂' },
  { to: '/users', label: '用户管理', icon: '♙' },
  { to: '/admins', label: '管理员', icon: '♜' },
]
onMounted(() => void session.loadProfile())
</script>

<template>
  <div class="app-frame">
    <aside class="sidebar" :class="{ open: mobileOpen }">
      <div class="brand">
        <span class="brand-mark">G</span>
        <div><strong>GRPC Console</strong><small>服务管理中心</small></div>
      </div>
      <nav>
        <RouterLink v-for="link in links" :key="link.to" :to="link.to" :class="{ active: route.path === link.to }" @click="mobileOpen = false">
          <span class="nav-icon">{{ link.icon }}</span>{{ link.label }}
        </RouterLink>
      </nav>
      <div class="sidebar-foot"><span class="status-dot"></span><span>本地开发环境</span></div>
    </aside>
    <div v-if="mobileOpen" class="backdrop" @click="mobileOpen = false"></div>

    <main class="main-area">
      <header class="topbar">
        <button class="menu-button" aria-label="打开菜单" @click="mobileOpen = !mobileOpen">☰</button>
        <div class="environment"><span></span>系统运行正常</div>
        <div class="account">
          <div class="avatar">{{ initials }}</div>
          <div class="account-copy"><strong>{{ session.state.profile?.username || '加载中…' }}</strong><small>{{ roleName }}</small></div>
          <button class="text-button" @click="logout">退出</button>
        </div>
      </header>
      <section class="page"><RouterView /></section>
    </main>
  </div>
</template>

