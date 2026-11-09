import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';

/** Web OAuth 回调占位页；AuthSession 完成回调后会自动关闭登录窗口。 */
export default function OAuthCallbackScreen() {
  return (
    <View style={styles.container}>
      <ActivityIndicator size="large" />
      <Text style={styles.text}>正在完成登录…</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 16 },
  text: { color: '#475569', fontSize: 16 },
});
