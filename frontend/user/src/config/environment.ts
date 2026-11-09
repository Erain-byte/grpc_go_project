const trimTrailingSlash = (value: string) => value.replace(/\/+$/, '');

/** 前端运行环境配置。生产构建必须通过 EXPO_PUBLIC_* 变量覆盖默认值。 */
export const environment = {
  keycloakIssuer: trimTrailingSlash(
    process.env.EXPO_PUBLIC_KEYCLOAK_ISSUER ??
      'http://localhost:8180/realms/grpc-project',
  ),
  keycloakClientId: process.env.EXPO_PUBLIC_KEYCLOAK_CLIENT_ID ?? 'user-app',
  gatewayURL: trimTrailingSlash(
    process.env.EXPO_PUBLIC_GATEWAY_URL ?? 'http://localhost:9080',
  ),
} as const;
