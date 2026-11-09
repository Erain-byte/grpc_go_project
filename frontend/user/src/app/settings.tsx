import Constants from 'expo-constants';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import { PageShell } from '@/components/page-shell';
import { clearAllLocalData } from '@/storage/local-data';

export default function SettingsScreen() {
  const router = useRouter();
  const [confirmClear, setConfirmClear] = useState(false);
  const [cleared, setCleared] = useState(false);

  const clear = async () => {
    await clearAllLocalData();
    setConfirmClear(false);
    setCleared(true);
  };

  return (
    <PageShell showSettings={false}>
      <Pressable onPress={() => router.back()}><Text style={styles.back}>‹ 返回</Text></Pressable>
      <Text style={styles.title}>设置与说明</Text>
      <Text style={styles.subtitle}>管理本机数据，并了解钱包体检的能力边界。</Text>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>隐私说明</Text>
        <Text style={styles.body}>查询对象是公开的 TRON 地址。应用不会读取、上传或保存私钥、助记词，也不会要求用户签名或连接钱包。</Text>
        <Text style={styles.body}>收藏地址、备注和最近一次结果仅保存在当前设备。服务端会按安全与运维要求记录必要的请求日志，但不建立用户账户。</Text>
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>服务范围</Text>
        <View style={styles.row}><Text style={styles.label}>网络</Text><Text style={styles.value}>TRON Mainnet</Text></View>
        <View style={styles.row}><Text style={styles.label}>代币</Text><Text style={styles.value}>官方 USDT 合约</Text></View>
        <View style={styles.row}><Text style={styles.label}>操作能力</Text><Text style={styles.value}>只读查询</Text></View>
        <Text style={styles.notice}>本工具不支持撤销授权、转账、提现或代替用户发起任何链上交易。风险标签和第三方数据可能延迟或暂时不可用。</Text>
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>本地数据</Text>
        <Text style={styles.body}>清除后会删除收藏地址、备注、最近结果和首次使用提示状态，操作不可恢复。</Text>
        <Pressable onPress={() => setConfirmClear(true)} style={styles.dangerButton}><Text style={styles.dangerText}>清除全部本地数据</Text></Pressable>
        {cleared ? <Text style={styles.success}>本地数据已清除。</Text> : null}
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>关于</Text>
        <View style={styles.row}><Text style={styles.label}>应用版本</Text><Text style={styles.value}>{Constants.expoConfig?.version ?? '1.0.0'}</Text></View>
        <View style={styles.row}><Text style={styles.label}>产品阶段</Text><Text style={styles.value}>A 包 · 钱包体检</Text></View>
        <Text style={styles.body}>问题反馈入口将在正式发布前接入。反馈时请提供请求编号，不要发送私钥、助记词或完整资产信息。</Text>
      </View>

      <Modal visible={confirmClear} transparent animationType="fade">
        <View style={styles.overlay}><View style={styles.modal}>
          <Text style={styles.modalTitle}>确认清除本地数据？</Text>
          <Text style={styles.body}>收藏地址和备注等数据将从当前设备永久删除。</Text>
          <View style={styles.actions}>
            <Pressable onPress={() => setConfirmClear(false)} style={styles.cancel}><Text style={styles.cancelText}>取消</Text></Pressable>
            <Pressable onPress={() => void clear()} style={styles.confirm}><Text style={styles.confirmText}>确认清除</Text></Pressable>
          </View>
        </View></View>
      </Modal>
    </PageShell>
  );
}

const styles = StyleSheet.create({
  back: { color: '#2563eb', fontWeight: '800' },
  title: { marginTop: 18, color: '#0f172a', fontSize: 28, fontWeight: '900' },
  subtitle: { marginTop: 7, marginBottom: 18, color: '#64748b' },
  card: { marginBottom: 12, padding: 20, borderRadius: 17, borderWidth: 1, borderColor: '#e2e8f0', backgroundColor: '#ffffff' },
  cardTitle: { marginBottom: 10, color: '#0f172a', fontSize: 16, fontWeight: '900' },
  body: { marginTop: 5, color: '#64748b', fontSize: 13, lineHeight: 21 },
  row: { paddingVertical: 10, flexDirection: 'row', justifyContent: 'space-between', gap: 16, borderBottomWidth: 1, borderBottomColor: '#f1f5f9' },
  label: { color: '#64748b' }, value: { color: '#0f172a', fontWeight: '800', textAlign: 'right' },
  notice: { marginTop: 13, padding: 13, color: '#92400e', fontSize: 12, lineHeight: 19, borderRadius: 10, backgroundColor: '#fffbeb' },
  dangerButton: { marginTop: 15, alignSelf: 'flex-start', paddingHorizontal: 16, paddingVertical: 11, borderRadius: 10, borderWidth: 1, borderColor: '#fecaca', backgroundColor: '#fff1f2' },
  dangerText: { color: '#dc2626', fontWeight: '800' }, success: { marginTop: 10, color: '#15803d', fontSize: 12, fontWeight: '700' },
  overlay: { flex: 1, padding: 20, justifyContent: 'center', alignItems: 'center', backgroundColor: 'rgba(15,23,42,0.5)' }, modal: { width: '100%', maxWidth: 440, padding: 23, borderRadius: 18, backgroundColor: '#fff' }, modalTitle: { color: '#0f172a', fontSize: 19, fontWeight: '900' }, actions: { marginTop: 20, flexDirection: 'row', justifyContent: 'flex-end', gap: 9 }, cancel: { paddingHorizontal: 18, paddingVertical: 11, borderWidth: 1, borderColor: '#cbd5e1', borderRadius: 10 }, cancelText: { color: '#475569', fontWeight: '800' }, confirm: { paddingHorizontal: 18, paddingVertical: 11, borderRadius: 10, backgroundColor: '#dc2626' }, confirmText: { color: '#fff', fontWeight: '800' },
});
