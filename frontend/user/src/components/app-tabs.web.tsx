import { Slot } from 'expo-router';

/**
 * Web 端路由出口。
 *
 * 业务导航尚未确定，因此这里先保持最小实现；后续增加底部导航时，
 * 再统一补充 Web 与 iOS 的导航结构。
 */
export default function AppTabs() {
  return <Slot />;
}
