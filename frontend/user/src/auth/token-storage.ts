import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

const tokenKey = 'web3-user.oauth-token';

/**
 * 原生端使用系统安全存储。
 * Web 端禁止持久化 Access/Refresh/ID Token，Token 只保留在 React 内存中；
 * 页面刷新后重新登录，后续生产 Web 可升级为 BFF + HttpOnly Cookie。
 */
export async function loadToken(): Promise<string | null> {
  if (Platform.OS === 'web') {
    return null;
  }

  return SecureStore.getItemAsync(tokenKey);
}

export async function saveToken(value: string): Promise<void> {
  if (Platform.OS === 'web') {
    return;
  }

  await SecureStore.setItemAsync(tokenKey, value);
}

export async function removeToken(): Promise<void> {
  if (Platform.OS === 'web') {
    return;
  }

  await SecureStore.deleteItemAsync(tokenKey);
}
