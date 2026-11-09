import { useRouter } from 'expo-router';
import type { PropsWithChildren } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

type PageShellProps = PropsWithChildren<{ showSettings?: boolean; scroll?: boolean }>;

/** 钱包体检各页面共用的浅色金融工具外壳。 */
export function PageShell({ children, showSettings = true, scroll = true }: PageShellProps) {
  const router = useRouter();
  const body = <View style={styles.content}>{children}</View>;

  return (
    <SafeAreaView style={styles.safeArea}>
      <View style={styles.header}>
        <Pressable onPress={() => router.replace('/')} style={styles.brand}>
          <View style={styles.logo}><Text style={styles.logoText}>✓</Text></View>
          <View>
            <Text style={styles.brandName}>钱包体检</Text>
            <Text style={styles.brandCaption}>TRON USDT 授权查询</Text>
          </View>
        </Pressable>
        {showSettings ? (
          <Pressable onPress={() => router.push('/settings')} style={styles.settingsButton}>
            <Text style={styles.settingsText}>设置</Text>
          </Pressable>
        ) : null}
      </View>
      {scroll ? <ScrollView contentContainerStyle={styles.scroll}>{body}</ScrollView> : body}
      <View style={styles.footer}>
        <Text style={styles.footerText}>
          仅查询公开地址的 TRON USDT 授权，不读取私钥，也不代表完整安全审计。
        </Text>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: '#f4f7fb' },
  header: { minHeight: 76, paddingHorizontal: 20, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', borderBottomWidth: 1, borderBottomColor: '#e2e8f0', backgroundColor: '#ffffff' },
  brand: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  logo: { width: 42, height: 42, borderRadius: 14, alignItems: 'center', justifyContent: 'center', backgroundColor: '#2563eb' },
  logoText: { color: '#ffffff', fontSize: 22, fontWeight: '900' },
  brandName: { color: '#0f172a', fontSize: 17, fontWeight: '800' },
  brandCaption: { marginTop: 2, color: '#64748b', fontSize: 11 },
  settingsButton: { borderRadius: 10, borderWidth: 1, borderColor: '#cbd5e1', paddingHorizontal: 15, paddingVertical: 9 },
  settingsText: { color: '#334155', fontWeight: '700' },
  scroll: { flexGrow: 1 },
  content: { flex: 1, width: '100%', maxWidth: 920, alignSelf: 'center', padding: 20 },
  footer: { paddingHorizontal: 24, paddingBottom: 18, alignItems: 'center' },
  footerText: { color: '#94a3b8', fontSize: 11, textAlign: 'center', lineHeight: 17 },
});
