const baseURL = (import.meta.env.VITE_ADMIN_BFF_URL ?? 'http://localhost:4181').replace(/\/+$/, '')

export class APIError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

function loginURL(): string {
  const returnURL = `${window.location.origin}${import.meta.env.BASE_URL}`
  return `${baseURL}/oauth2/start?rd=${encodeURIComponent(returnURL)}`
}

export function beginLogin(): void {
  window.location.assign(loginURL())
}

export function logout(): void {
  const returnURL = `${window.location.origin}${import.meta.env.BASE_URL}`
  window.location.assign(`${baseURL}/oauth2/sign_out?rd=${encodeURIComponent(returnURL)}`)
}

export async function postJSON<T>(path: string, body: unknown): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${baseURL}${path}`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  } catch {
    // 未登录时 OAuth2 Proxy 会重定向到 Keycloak。跨域 fetch 可能在重定向阶段
    // 被浏览器拦截，因此这里也主动进入登录流程。
    beginLogin()
    throw new APIError('无法确认登录状态，正在跳转登录', 0)
  }

  if (response.status === 401 || response.status === 403) {
    beginLogin()
    throw new APIError('登录状态无效，正在跳转登录', response.status)
  }
  const payload = await response.json().catch(() => ({})) as Record<string, unknown>
  if (!response.ok) {
    const message = typeof payload.error === 'string'
      ? payload.error
      : typeof payload.message === 'string' ? payload.message : `请求失败（HTTP ${response.status}）`
    throw new APIError(message, response.status)
  }
  return payload as T
}

