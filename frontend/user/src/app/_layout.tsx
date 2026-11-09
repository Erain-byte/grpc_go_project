import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

/** Web 预览与后续 iOS 共用同一套路由。 */
export default function RootLayout() {
  return (
    <>
      <Stack screenOptions={{ headerShown: false }} />
      <StatusBar style="dark" />
    </>
  );
}
