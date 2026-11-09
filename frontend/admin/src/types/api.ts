export type AdminRole = 0 | 1 | 2

export interface AdminInfo {
  id: string
  username: string
  email: string
  role: AdminRole
}

export interface UserInfo {
  id: string
  username: string
  email?: string
  nickname?: string
  phone?: string
  avatar_url?: string
  status: 0 | 1 | 2
  created_at?: { seconds?: string | number; nanos?: number }
  updated_at?: { seconds?: string | number; nanos?: number }
  last_login_at?: { seconds?: string | number; nanos?: number }
}

export interface AdminInfoResponse {
  success: boolean
  message: string
  admin_info: AdminInfo
}

export interface AdminListResponse {
  admins: AdminInfo[]
  total: number
  page: number
  page_size: number
}

export interface UserListResponse {
  success: boolean
  message: string
  user_list: UserInfo[]
  total: number
  page: number
  page_size: number
}

export interface CreateAdminRequest {
  username: string
  password: string
  email: string
  role: 1 | 2
}

