import AsyncStorage from '@react-native-async-storage/async-storage';
import type { ApprovalResult, FavoriteAddress } from '@/types/approval';

const favoritesKey = 'wallet-check.favorites.v1';
const onboardingKey = 'wallet-check.onboarding.v1';
const installIDKey = 'wallet-check.install-id.v1';
export const favoriteLimit = 3;
export const noteLimit = 20;

export async function hasCompletedOnboarding(): Promise<boolean> {
  return (await AsyncStorage.getItem(onboardingKey)) === 'true';
}
export async function completeOnboarding(): Promise<void> {
  await AsyncStorage.setItem(onboardingKey, 'true');
}
export async function getFavorites(): Promise<FavoriteAddress[]> {
  const payload = await AsyncStorage.getItem(favoritesKey);
  if (!payload) return [];
  try {
    const parsed = JSON.parse(payload) as FavoriteAddress[];
    return Array.isArray(parsed) ? parsed.slice(0, favoriteLimit) : [];
  } catch { return []; }
}
export async function saveFavorite(address: string, note: string, lastResult?: ApprovalResult): Promise<FavoriteAddress[]> {
  const favorites = await getFavorites();
  const index = favorites.findIndex((item) => item.address === address);
  const next: FavoriteAddress = { network: 'tron', address, note: note.trim().slice(0, noteLimit), updatedAt: new Date().toISOString(), lastResult: lastResult ?? (index >= 0 ? favorites[index].lastResult : undefined) };
  if (index >= 0) favorites[index] = next;
  else {
    if (favorites.length >= favoriteLimit) throw new Error(`最多收藏 ${favoriteLimit} 个地址`);
    favorites.unshift(next);
  }
  await AsyncStorage.setItem(favoritesKey, JSON.stringify(favorites));
  return favorites;
}
export async function removeFavorite(address: string): Promise<FavoriteAddress[]> {
  const favorites = (await getFavorites()).filter((item) => item.address !== address);
  await AsyncStorage.setItem(favoritesKey, JSON.stringify(favorites));
  return favorites;
}
export async function saveLatestResult(result: ApprovalResult): Promise<void> {
  const favorites = await getFavorites();
  const index = favorites.findIndex((item) => item.address === result.address);
  if (index < 0) return;
  favorites[index] = { ...favorites[index], lastResult: result, updatedAt: new Date().toISOString() };
  await AsyncStorage.setItem(favoritesKey, JSON.stringify(favorites));
}
export async function clearAllLocalData(): Promise<void> {
  await AsyncStorage.multiRemove([favoritesKey, onboardingKey, installIDKey]);
}
