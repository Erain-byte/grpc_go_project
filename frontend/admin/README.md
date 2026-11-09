# Admin 管理后台

Vue 3 + TypeScript + Vite 管理后台。认证通过 `admin-web` Keycloak Client 和 Admin OAuth2 Proxy 完成，浏览器 JavaScript 不读取或保存 Access Token、Refresh Token。

## 本地启动

先启动 Keycloak、Redis、OAuth2 Proxy、Gateway、Admin 和 User 服务，然后执行：

```powershell
cd D:\grpc_go_project\frontend\admin
Copy-Item .env.example .env.local
npm.cmd install
npm.cmd run dev
```

访问 `http://localhost:3000`。未登录时页面会跳转到 `http://localhost:4181/oauth2/start`，登录成功后返回管理后台。

## 当前功能

- 管理概览；
- 当前管理员资料；
- 用户分页列表；
- 管理员分页列表；
- 超级管理员创建管理员；
- OAuth2 Proxy 注入 Access Token，Gateway 和 Admin 分别验证权限。

## 构建检查

```powershell
npm.cmd run build
```

生产环境必须把 `VITE_ADMIN_BFF_URL`、Keycloak 回调地址、Cookie Secure 和代理域名切换为正式 HTTPS 地址。

