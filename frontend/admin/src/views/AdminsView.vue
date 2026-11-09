<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { adminAPI } from '../services/admin-api'
import { useSession } from '../stores/session'
import type { AdminInfo, CreateAdminRequest } from '../types/api'

const session = useSession()
const admins = ref<AdminInfo[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const error = ref('')
const success = ref('')
const showCreate = ref(false)
const submitting = ref(false)
const form = reactive<CreateAdminRequest>({ username: '', email: '', password: '', role: 2 })
const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

function roleName(role: number) { return role === 1 ? '超级管理员' : role === 2 ? '管理员' : '未分配' }
async function load(target = page.value) {
  loading.value = true; error.value = ''
  try { const response = await adminAPI.admins(target, pageSize); admins.value = response.admins ?? []; total.value = response.total; page.value = response.page || target }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '管理员列表加载失败' }
  finally { loading.value = false }
}
async function createAdmin() {
  submitting.value = true; error.value = ''; success.value = ''
  try {
    await adminAPI.createAdmin({ ...form })
    showCreate.value = false; success.value = `管理员 ${form.username} 已创建，可使用 Keycloak 登录。`
    Object.assign(form, { username: '', email: '', password: '', role: 2 })
    await load(1)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '管理员创建失败' }
  finally { submitting.value = false }
}
onMounted(() => { void session.loadProfile(); void load() })
</script>

<template>
  <div class="page-heading"><div><span class="eyebrow">ACCESS CONTROL</span><h1>管理员</h1><p>管理员身份同时创建于 Keycloak 和本地业务数据库。</p></div><button v-if="session.isSuperAdmin.value" class="primary-button" @click="showCreate = true">＋ 新建管理员</button></div>
  <div v-if="error" class="alert error">{{ error }}</div><div v-if="success" class="alert success">{{ success }}</div>
  <section class="panel table-panel">
    <div class="panel-head"><div><h2>管理员列表</h2><p>共 {{ total }} 个账号</p></div><span class="secure-label">Keycloak 已绑定</span></div>
    <div class="table-scroll"><table><thead><tr><th>管理员</th><th>邮箱</th><th>角色</th><th>身份标识</th></tr></thead><tbody>
      <tr v-if="loading"><td colspan="4" class="empty-cell">正在加载管理员…</td></tr>
      <tr v-else-if="admins.length === 0"><td colspan="4" class="empty-cell">暂无管理员记录</td></tr>
      <tr v-for="admin in admins" :key="admin.id"><td><div class="identity-cell"><span class="mini-avatar admin">{{ admin.username.slice(0, 1).toUpperCase() }}</span><strong>{{ admin.username }}</strong></div></td><td>{{ admin.email || '—' }}</td><td><span class="role-badge" :class="{ super: admin.role === 1 }">{{ roleName(admin.role) }}</span></td><td><code>{{ admin.id }}</code></td></tr>
    </tbody></table></div>
    <div class="pagination"><span>第 {{ page }} / {{ pages }} 页</span><div><button :disabled="page <= 1 || loading" @click="load(page - 1)">上一页</button><button :disabled="page >= pages || loading" @click="load(page + 1)">下一页</button></div></div>
  </section>

  <div v-if="showCreate" class="modal-backdrop" @click.self="showCreate = false"><form class="modal" @submit.prevent="createAdmin"><div class="modal-head"><div><span class="eyebrow">NEW ADMIN</span><h2>创建管理员</h2></div><button type="button" class="close-button" @click="showCreate = false">×</button></div><p class="modal-note">提交后会同时创建 Keycloak 登录身份和 PostgreSQL 本地资料。</p>
    <label>用户名<input v-model.trim="form.username" required minlength="3" maxlength="32" autocomplete="off" placeholder="operator01"></label>
    <label>邮箱<input v-model.trim="form.email" required type="email" autocomplete="off" placeholder="operator@example.com"></label>
    <label>初始密码<input v-model="form.password" required type="password" minlength="8" maxlength="128" autocomplete="new-password" placeholder="至少 8 个字符"></label>
    <label>角色<select v-model="form.role"><option :value="2">管理员</option><option :value="1">超级管理员</option></select></label>
    <div class="modal-actions"><button type="button" class="secondary-button" @click="showCreate = false">取消</button><button class="primary-button" :disabled="submitting">{{ submitting ? '正在创建…' : '确认创建' }}</button></div>
  </form></div>
</template>

