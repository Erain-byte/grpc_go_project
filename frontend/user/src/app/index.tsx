import * as Clipboard from 'expo-clipboard';
import { useFocusEffect, useRouter } from 'expo-router';
import { useCallback, useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { PageShell } from '@/components/page-shell';
import { completeOnboarding, favoriteLimit, getFavorites, hasCompletedOnboarding, noteLimit, removeFavorite, saveFavorite } from '@/storage/local-data';
import type { FavoriteAddress } from '@/types/approval';
import { truncateAddress, validateTronAddress } from '@/utils/tron-address';

function resultText(item: FavoriteAddress): string {
  const result = item.lastResult;
  if (!result) return '尚未检测';
  if (result.status === 1) return result.approvals?.length ? `发现 ${result.approvals.length} 项授权` : '未发现授权';
  if (result.status === 2) return '结果不完整';
  return '检测失败';
}

export default function HomeScreen() {
  const router = useRouter();
  const [address, setAddress] = useState('');
  const [error, setError] = useState('');
  const [checking, setChecking] = useState(false);
  const [favorites, setFavorites] = useState<FavoriteAddress[]>([]);
  const [showOnboarding, setShowOnboarding] = useState(false);
  const [editing, setEditing] = useState<FavoriteAddress | null>(null);
  const [note, setNote] = useState('');

  const reload = useCallback(() => {
    void Promise.all([getFavorites(), hasCompletedOnboarding()]).then(([items, completed]) => {
      setFavorites(items);
      setShowOnboarding(!completed);
    });
  }, []);
  useFocusEffect(reload);

  const submit = async (value = address) => {
    if (checking) return;
    setChecking(true);
    setError('');
    const normalized = value.trim();
    const validationError = await validateTronAddress(normalized);
    setChecking(false);
    if (validationError) { setError(validationError); return; }
    router.push({ pathname: '/result', params: { address: normalized } });
  };

  const paste = async () => {
    const value = (await Clipboard.getStringAsync()).trim();
    setAddress(value);
    setError('');
  };

  return (
    <PageShell>
      <View style={styles.hero}>
        <View style={styles.eyebrow}><Text style={styles.eyebrowText}>TRON MAINNET · OFFICIAL USDT</Text></View>
        <Text style={styles.title}>检查公开地址的 USDT 授权</Text>
        <Text style={styles.subtitle}>无需登录，不连接钱包。输入公开地址即可查看授权对象、额度和风险提示。</Text>
        <View style={styles.inputRow}>
          <TextInput value={address} onChangeText={(value) => { setAddress(value); setError(''); }} placeholder="输入 T 开头的 TRON 地址" placeholderTextColor="#94a3b8" autoCapitalize="none" autoCorrect={false} style={styles.input} />
          <Pressable onPress={paste} style={styles.pasteButton}><Text style={styles.pasteText}>粘贴</Text></Pressable>
        </View>
        {error ? <Text style={styles.error}>{error}</Text> : null}
        <Pressable onPress={() => void submit()} disabled={checking} style={[styles.primaryButton, checking && styles.disabled]}>
          <Text style={styles.primaryText}>{checking ? '正在校验地址…' : '开始体检'}</Text>
        </Pressable>
        <Text style={styles.hint}>系统会执行完整 Base58Check 校验；不会请求私钥、助记词或签名。</Text>
      </View>

      <View style={styles.sectionHeader}>
        <View><Text style={styles.sectionTitle}>常用地址</Text><Text style={styles.sectionCaption}>仅保存在当前设备，最多 {favoriteLimit} 个</Text></View>
        <Text style={styles.counter}>{favorites.length}/{favoriteLimit}</Text>
      </View>
      {favorites.length === 0 ? (
        <View style={styles.empty}><Text style={styles.emptyIcon}>☆</Text><Text style={styles.emptyTitle}>还没有收藏地址</Text><Text style={styles.emptyText}>完成一次检测后，可以把该公开地址收藏到本机。</Text></View>
      ) : favorites.map((item) => (
        <View key={item.address} style={styles.favoriteCard}>
          <View style={styles.favoriteMain}>
            <Text style={styles.note}>{item.note || '未命名地址'}</Text>
            <Text style={styles.address}>{truncateAddress(item.address, 10, 8)}</Text>
            <Text style={styles.result}>{resultText(item)}</Text>
          </View>
          <View style={styles.actions}>
            <Pressable onPress={() => void submit(item.address)} style={styles.smallPrimary}><Text style={styles.smallPrimaryText}>重新检测</Text></Pressable>
            <Pressable onPress={() => { setEditing(item); setNote(item.note); }} style={styles.smallButton}><Text style={styles.smallText}>备注</Text></Pressable>
            <Pressable onPress={() => void removeFavorite(item.address).then(setFavorites)} style={styles.smallButton}><Text style={styles.dangerText}>删除</Text></Pressable>
          </View>
        </View>
      ))}

      <Modal visible={showOnboarding} transparent animationType="fade">
        <View style={styles.overlay}><View style={styles.modal}>
          <Text style={styles.modalIcon}>✓</Text><Text style={styles.modalTitle}>开始前请了解</Text>
          <Text style={styles.modalText}>本工具只查询链上公开数据，不要求登录，也不会读取私钥、助记词或发起交易。</Text>
          <Text style={styles.modalText}>风险标签来自外部数据源，仅用于辅助判断；“未发现授权”不等于地址绝对安全。</Text>
          <Pressable onPress={() => void completeOnboarding().then(() => setShowOnboarding(false))} style={styles.primaryButton}><Text style={styles.primaryText}>我已了解</Text></Pressable>
        </View></View>
      </Modal>

      <Modal visible={editing !== null} transparent animationType="fade">
        <View style={styles.overlay}><View style={styles.modal}>
          <Text style={styles.modalTitle}>修改地址备注</Text>
          <TextInput value={note} onChangeText={setNote} maxLength={noteLimit} placeholder="例如：日常钱包" style={styles.noteInput} />
          <Text style={styles.countText}>{note.length}/{noteLimit}</Text>
          <View style={styles.modalActions}>
            <Pressable onPress={() => setEditing(null)} style={styles.cancelButton}><Text style={styles.smallText}>取消</Text></Pressable>
            <Pressable onPress={() => editing && void saveFavorite(editing.address, note).then((items) => { setFavorites(items); setEditing(null); })} style={styles.saveButton}><Text style={styles.primaryText}>保存</Text></Pressable>
          </View>
        </View></View>
      </Modal>
    </PageShell>
  );
}

const styles = StyleSheet.create({
  hero: { backgroundColor: '#ffffff', borderRadius: 22, padding: 24, borderWidth: 1, borderColor: '#e2e8f0' },
  eyebrow: { alignSelf: 'flex-start', backgroundColor: '#eff6ff', borderRadius: 99, paddingHorizontal: 11, paddingVertical: 6 },
  eyebrowText: { color: '#2563eb', fontSize: 11, fontWeight: '800' },
  title: { marginTop: 18, color: '#0f172a', fontSize: 28, lineHeight: 36, fontWeight: '900' },
  subtitle: { marginTop: 10, color: '#64748b', fontSize: 14, lineHeight: 22 },
  inputRow: { marginTop: 24, flexDirection: 'row', gap: 10 },
  input: { flex: 1, minWidth: 0, borderWidth: 1, borderColor: '#cbd5e1', borderRadius: 12, paddingHorizontal: 14, paddingVertical: 13, color: '#0f172a', backgroundColor: '#f8fafc' },
  pasteButton: { justifyContent: 'center', borderRadius: 12, borderWidth: 1, borderColor: '#bfdbfe', paddingHorizontal: 17, backgroundColor: '#eff6ff' },
  pasteText: { color: '#2563eb', fontWeight: '800' },
  error: { marginTop: 9, color: '#dc2626', fontSize: 12 },
  primaryButton: { marginTop: 15, minHeight: 48, borderRadius: 12, alignItems: 'center', justifyContent: 'center', backgroundColor: '#2563eb', paddingHorizontal: 20 },
  primaryText: { color: '#ffffff', fontWeight: '800' },
  disabled: { opacity: 0.55 },
  hint: { marginTop: 10, color: '#94a3b8', fontSize: 11, lineHeight: 17 },
  sectionHeader: { marginTop: 28, marginBottom: 12, flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-end' },
  sectionTitle: { color: '#0f172a', fontSize: 18, fontWeight: '800' },
  sectionCaption: { marginTop: 4, color: '#64748b', fontSize: 12 },
  counter: { color: '#64748b', fontWeight: '700' },
  empty: { alignItems: 'center', padding: 30, borderWidth: 1, borderStyle: 'dashed', borderColor: '#cbd5e1', borderRadius: 18, backgroundColor: '#ffffff' },
  emptyIcon: { color: '#2563eb', fontSize: 32 }, emptyTitle: { marginTop: 8, color: '#334155', fontWeight: '800' }, emptyText: { marginTop: 5, color: '#94a3b8', fontSize: 12, textAlign: 'center' },
  favoriteCard: { marginBottom: 10, padding: 17, flexDirection: 'row', flexWrap: 'wrap', gap: 14, justifyContent: 'space-between', alignItems: 'center', borderRadius: 16, borderWidth: 1, borderColor: '#e2e8f0', backgroundColor: '#ffffff' },
  favoriteMain: { flexGrow: 1 }, note: { color: '#0f172a', fontWeight: '800' }, address: { marginTop: 4, color: '#64748b', fontSize: 12 }, result: { marginTop: 7, color: '#2563eb', fontSize: 12, fontWeight: '700' },
  actions: { flexDirection: 'row', gap: 7, flexWrap: 'wrap' }, smallPrimary: { borderRadius: 9, backgroundColor: '#2563eb', paddingHorizontal: 12, paddingVertical: 9 }, smallPrimaryText: { color: '#fff', fontSize: 12, fontWeight: '700' }, smallButton: { borderRadius: 9, borderWidth: 1, borderColor: '#cbd5e1', paddingHorizontal: 11, paddingVertical: 8 }, smallText: { color: '#475569', fontSize: 12, fontWeight: '700' }, dangerText: { color: '#dc2626', fontSize: 12, fontWeight: '700' },
  overlay: { flex: 1, padding: 20, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(15,23,42,0.5)' }, modal: { width: '100%', maxWidth: 480, borderRadius: 20, padding: 24, backgroundColor: '#ffffff' }, modalIcon: { color: '#16a34a', fontSize: 34, fontWeight: '900' }, modalTitle: { marginTop: 7, marginBottom: 9, color: '#0f172a', fontSize: 20, fontWeight: '900' }, modalText: { marginTop: 8, color: '#475569', lineHeight: 21 }, noteInput: { marginTop: 12, borderWidth: 1, borderColor: '#cbd5e1', borderRadius: 10, padding: 12, color: '#0f172a' }, countText: { marginTop: 5, color: '#94a3b8', fontSize: 11, textAlign: 'right' }, modalActions: { marginTop: 15, flexDirection: 'row', justifyContent: 'flex-end', gap: 9 }, cancelButton: { paddingHorizontal: 18, paddingVertical: 12, borderWidth: 1, borderColor: '#cbd5e1', borderRadius: 10 }, saveButton: { paddingHorizontal: 22, paddingVertical: 12, backgroundColor: '#2563eb', borderRadius: 10 },
});
