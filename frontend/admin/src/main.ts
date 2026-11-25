import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import { useSession } from './stores/session'
import './style.css'

async function bootstrap() {
  // Vue 挂载前先调用 /admin/info，同时校验 OAuth2 会话和管理员身份。
  // 401 由请求层跳转 OAuth2 登录；其他错误直接显示，不挂载后台。
  const session = useSession()
  const authenticated = await session.loadProfile()
  if (!authenticated) {
    const root = document.querySelector<HTMLElement>('#app')
    if (root && session.state.error) root.textContent = session.state.error
    return
  }

  createApp(App).use(router).mount('#app')
}

void bootstrap()

