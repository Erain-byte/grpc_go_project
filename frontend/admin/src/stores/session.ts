import { computed, reactive, readonly } from 'vue'
import { adminAPI } from '../services/admin-api'
import type { AdminInfo } from '../types/api'

const state = reactive<{ profile: AdminInfo | null; loading: boolean; error: string }>({
  profile: null,
  loading: false,
  error: '',
})

export function useSession() {
  async function loadProfile(force = false) {
    if ((state.profile && !force) || state.loading) return
    state.loading = true
    state.error = ''
    try {
      state.profile = (await adminAPI.profile()).admin_info
    } catch (error) {
      state.error = error instanceof Error ? error.message : '管理员信息加载失败'
    } finally {
      state.loading = false
    }
  }

  return {
    state: readonly(state),
    isSuperAdmin: computed(() => state.profile?.role === 1),
    loadProfile,
  }
}

