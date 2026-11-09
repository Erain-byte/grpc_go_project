<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { adminAPI } from '../services/admin-api'

const userTotal = ref<number | null>(null)
const adminTotal = ref<number | null>(null)
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    const [users, admins] = await Promise.all([adminAPI.users(1, 1), adminAPI.admins(1, 1)])
    userTotal.value = users.total
    adminTotal.value = admins.total
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '统计数据加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page-heading">
    <div><span class="eyebrow">ADMIN OVERVIEW</span><h1>管理概览</h1><p>查看账号规模和核心服务入口。</p></div>
    <div class="date-chip">{{ new Date().toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' }) }}</div>
  </div>
  <div v-if="error" class="alert error">{{ error }}</div>
  <div class="metric-grid">
    <article class="metric-card blue"><div class="metric-icon">♙</div><div><span>业务用户</span><strong>{{ loading ? '—' : userTotal }}</strong><RouterLink to="/users">查看用户 ›</RouterLink></div></article>
    <article class="metric-card violet"><div class="metric-icon">♜</div><div><span>管理员账号</span><strong>{{ loading ? '—' : adminTotal }}</strong><RouterLink to="/admins">查看管理员 ›</RouterLink></div></article>
    <article class="metric-card green"><div class="metric-icon">✓</div><div><span>认证状态</span><strong>Keycloak</strong><small>OAuth2 Proxy 会话保护</small></div></article>
  </div>
  <section class="panel welcome-panel">
    <div><span class="eyebrow">SECURITY FIRST</span><h2>后台操作通过统一身份认证</h2><p>浏览器只保存 HttpOnly Session Cookie。Access Token 由 OAuth2 Proxy 注入并由 Gateway 再次验证，Vue 页面不会读取或持久化 Token。</p></div>
    <div class="flow"><span>Browser</span><b>→</b><span>OAuth2 Proxy</span><b>→</b><span>Gateway</span><b>→</b><span>Admin</span></div>
  </section>
</template>

