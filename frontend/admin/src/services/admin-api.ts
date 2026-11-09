import { postJSON } from './http'
import type { AdminInfoResponse, AdminListResponse, CreateAdminRequest, UserListResponse } from '../types/api'

export const adminAPI = {
  profile: () => postJSON<AdminInfoResponse>('/admin/info', {}),
  admins: (page = 1, pageSize = 20) => postJSON<AdminListResponse>('/admin/list', { page, page_size: pageSize }),
  users: (page = 1, pageSize = 20) => postJSON<UserListResponse>('/admin/users/list', { page, page_size: pageSize }),
  createAdmin: (request: CreateAdminRequest) => postJSON('/admin/create', request),
}

