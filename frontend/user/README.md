# 钱包体检用户端（Expo Web / iOS）

该目录使用 Expo、React Native、TypeScript 和 Expo Router。当前优先开发 Web 预览版，页面与业务逻辑可继续用于 iOS 构建。

## 当前产品入口

A 包“钱包体检”不要求注册或登录。用户直接输入公开 TRON 地址，查询官方 USDT 合约授权情况。

- 首页：地址输入、粘贴、完整 Base58Check 校验、本地收藏；
- 结果页：展示完整、部分或失败状态，授权额度和风险提示；
- 设置页：隐私与能力边界、本地数据清除；
- 收藏地址、备注和最近一次结果只保存在当前设备；
- `src/auth` 与 `src/app/oauth` 暂时保留，供以后需要账户体系的功能复用，不属于当前主流程。

## 本地启动

```powershell
cd D:\grpc_go_project\frontend\user
npm.cmd run web
```

默认地址：

- Expo Web：`http://localhost:8081`
- Gateway：`http://localhost:9080`

可以通过 `EXPO_PUBLIC_GATEWAY_URL` 覆盖 Gateway 地址。

## 基础检查

```powershell
npx.cmd tsc --noEmit
npm.cmd run lint
npx.cmd expo export --platform web
```

业务页面位于 `src/app`，公共组件位于 `src/components`。

## 安全边界

前端不会读取或保存私钥、助记词，也不会要求签名或发起链上交易。风险标签只用于辅助判断，“未发现授权”不代表完整安全审计。
