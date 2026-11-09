<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { adminAPI } from '../services/admin-api'
import type { UserInfo } from '../types/api'

const users = ref<UserInfo[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const error = ref('')

function formatTime(value?: { seconds?: string | number }) {
  if (!value?.seconds) return '—'
  return new Date(Number(value.seconds) * 1000).toLocaleString('zh-CN')
}
async function load(target = page.value) {
  loading.value = true; error.value = ''
  try {
    const response = await adminAPI.users(target, pageSize)
    users.value = response.user_list ?? []
    total.value = response.total
    page.value = response.page || target
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '用户列表加载失败' }
  finally { loading.value = false }
}
onMounted(() => void load())
</script>

<template>
  <div class="page-heading"><div><span class="eyebrow">USER DIRECTORY</span><h1>用户管理</h1><p>跨分表查询业务用户资料和账号状态。</p></div><button class="secondary-button" :disabled="loading" @click="load()">刷新列表</button></div>
  <div v-if="error" class="alert error">{{ error }}</div>
  <section class="panel table-panel">
    <div class="panel-head"><div><h2>用户列表</h2><p>共 {{ total }} 条记录</p></div><span class="read-only">只读视图</span></div>
    <div class="table-scroll">
      <table><thead><tr><th>用户</th><th>邮箱 / 手机</th><th>状态</th><th>创建时间</th><th>最近登录</th></tr></thead>
      <tbody>
        <tr v-if="loading"><td colspan="5" class="empty-cell">正在加载用户数据…</td></tr>
        <tr v-else-if="users.length === 0"><td colspan="5" class="empty-cell">暂无用户记录</td></tr>
        <tr v-for="user in users" :key="user.id">
          <td><div class="identity-cell"><span class="mini-avatar">{{ (user.username || 'U').slice(0, 1).toUpperCase() }}</span><div><strong>{{ user.nickname || user.username || '未命名用户' }}</strong><small>{{ user.id }}</small></div></div></td>
          <td><strong class="normal">{{ user.email || '—' }}</strong><small class="cell-sub">{{ user.phone || '未绑定手机' }}</small></td>
          <td><span class="status" :class="user.status === 1 ? 'active' : 'disabled'">{{ user.status === 1 ? '正常' : '已停用' }}</span></td>
          <td>{{ formatTime(user.created_at) }}</td><td>{{ formatTime(user.last_login_at) }}</td>
        </tr>
      </tbody></table>
    </div>
    <div class="pagination"><span>第 {{ page }} 页</span><div><button :disabled="page <= 1 || loading" @click="load(page - 1)">上一页</button><button :disabled="page * pageSize >= total || loading" @click="load(page + 1)">下一页</button></div></div>
  </section>
</template>

