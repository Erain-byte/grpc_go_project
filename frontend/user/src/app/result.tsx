import { useLocalSearchParams, useRouter } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Linking, Modal, Pressable, Share, StyleSheet, Text, TextInput, View } from 'react-native';

import { PageShell } from '@/components/page-shell';
import { checkTronApprovals } from '@/services/approval-service';
import { favoriteLimit, getFavorites, noteLimit, saveFavorite, saveLatestResult } from '@/storage/local-data';
import type { ApprovalResult, ProtoTimestamp, TronApproval } from '@/types/approval';
import { truncateAddress, validateTronAddress } from '@/utils/tron-address';

function formatTime(value?: ProtoTimestamp): string {
  if (!value?.seconds) return '时间未知';
  const seconds = typeof value.seconds === 'string' ? Number(value.seconds) : value.seconds;
  return Number.isFinite(seconds) ? new Date(seconds * 1000).toLocaleString('zh-CN') : '时间未知';
}
function statusMeta(result: ApprovalResult) {
  if (result.status === 1) return { text: '查询完成', color: '#15803d', background: '#dcfce7' };
  if (result.status === 2) return { text: '部分完成', color: '#b45309', background: '#fef3c7' };
  return { text: '查询失败', color: '#b91c1c', background: '#fee2e2' };
}
function spenderType(value?: number): string {
  return value === 1 ? '普通地址' : value === 2 ? '合约地址' : '类型未知';
}
function allowanceText(item: TronApproval): string {
  if (!item.allowance_verified) return '额度未核实';
  if (item.unlimited_status === 2) return '无限额度';
  if (item.unlimited_status === 3) return '额度状态冲突';
  return item.display_allowance || item.raw_allowance || '0';
}
function safeOfficialURL(value?: string): string | null {
  if (!value) return null;
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'https:' && (parsed.hostname === 'tronscan.org' || parsed.hostname.endsWith('.tronscan.org')) ? value : null;
  } catch { return null; }
}

export default function ResultScreen() {
  const router = useRouter();
  const params = useLocalSearchParams<{ address?: string }>();
  const address = typeof params.address === 'string' ? params.address : '';
  const [result, setResult] = useState<ApprovalResult | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [refreshKey, setRefreshKey] = useState(0);
  const [isFavorite, setIsFavorite] = useState(false);
  const [showFavorite, setShowFavorite] = useState(false);
  const [note, setNote] = useState('');
  const [favoriteError, setFavoriteError] = useState('');

  const load = useCallback(async (signal: AbortSignal) => {
    setLoading(true); setError(''); setResult(null);
    const invalid = await validateTronAddress(address);
    if (invalid) { setError(invalid); setLoading(false); return; }
    try {
      const data = await checkTronApprovals(address, signal);
      if (signal.aborted) return;
      setResult(data);
      await saveLatestResult(data);
      setIsFavorite((await getFavorites()).some((item) => item.address === address));
    } catch (cause) {
      if (!signal.aborted) setError(cause instanceof Error ? cause.message : '查询失败，请稍后重试');
    } finally { if (!signal.aborted) setLoading(false); }
  }, [address]);

  useEffect(() => {
    const controller = new AbortController();
    // 放到微任务中执行，避免在 effect 主体中同步触发一组状态更新。
    void Promise.resolve().then(() => load(controller.signal));
    return () => controller.abort();
  }, [load, refreshKey]);

  const openFavorite = async () => {
    const favorites = await getFavorites();
    const current = favorites.find((item) => item.address === address);
    if (!current && favorites.length >= favoriteLimit) { setFavoriteError(`最多收藏 ${favoriteLimit} 个地址，请先在首页删除一个`); return; }
    setNote(current?.note ?? ''); setFavoriteError(''); setShowFavorite(true);
  };
  const persistFavorite = async () => {
    if (!result) return;
    try { await saveFavorite(address, note, result); setIsFavorite(true); setShowFavorite(false); }
    catch (cause) { setFavoriteError(cause instanceof Error ? cause.message : '保存失败'); }
  };
  const share = async () => {
    if (!result) return;
    const meta = statusMeta(result);
    await Share.share({ message: `TRON USDT 授权体检：${truncateAddress(address)}，${meta.text}，共 ${result.approvals?.length ?? 0} 项。结果仅供参考。` });
  };

  const meta = result ? statusMeta(result) : null;
  const incomplete = result && (result.status === 2 || result.truncated || !result.enumeration_complete || !result.allowance_verification_complete);

  return (
    <PageShell showSettings={false}>
      <Pressable onPress={() => router.back()}><Text style={styles.back}>‹ 返回首页</Text></Pressable>
      {loading ? <View style={styles.stateCard}><Text style={styles.stateIcon}>◌</Text><Text style={styles.stateTitle}>正在查询链上授权</Text><Text style={styles.stateText}>正在核实授权对象、额度与风险数据，请稍候。</Text></View> : null}
      {!loading && error ? <View style={styles.errorCard}><Text style={styles.stateTitle}>查询未完成</Text><Text style={styles.stateText}>{error}</Text><Pressable onPress={() => setRefreshKey((value) => value + 1)} style={styles.primaryButton}><Text style={styles.primaryText}>重新查询</Text></Pressable></View> : null}
      {!loading && result && meta ? <>
        <View style={styles.summary}>
          <View style={styles.summaryTop}><View><Text style={styles.kicker}>检测结果</Text><Text style={styles.summaryAddress}>{truncateAddress(address, 12, 10)}</Text></View><View style={[styles.badge, { backgroundColor: meta.background }]}><Text style={[styles.badgeText, { color: meta.color }]}>{meta.text}</Text></View></View>
          <View style={styles.metrics}>
            <View style={styles.metric}><Text style={styles.metricValue}>{result.approvals?.length ?? 0}</Text><Text style={styles.metricLabel}>授权对象</Text></View>
            <View style={styles.metric}><Text style={styles.metricValue}>{result.network || 'TRON'}</Text><Text style={styles.metricLabel}>网络</Text></View>
            <View style={styles.metric}><Text style={styles.metricValue}>{result.is_cached ? '缓存' : '实时'}</Text><Text style={styles.metricLabel}>{formatTime(result.queried_at)}</Text></View>
          </View>
          <Text style={styles.reason}>{result.reason || '结果基于公开链上数据和已接入风险源。'}</Text>
          <View style={styles.toolbar}><Pressable onPress={() => setRefreshKey((value) => value + 1)} style={styles.outlineButton}><Text style={styles.outlineText}>刷新</Text></Pressable><Pressable onPress={() => void openFavorite()} style={styles.outlineButton}><Text style={styles.outlineText}>{isFavorite ? '修改收藏' : '收藏地址'}</Text></Pressable><Pressable onPress={() => void share()} style={styles.outlineButton}><Text style={styles.outlineText}>分享摘要</Text></Pressable></View>
          {favoriteError ? <Text style={styles.inlineError}>{favoriteError}</Text> : null}
        </View>

        {incomplete ? <View style={styles.warning}><Text style={styles.warningTitle}>当前结果不完整</Text><Text style={styles.warningText}>部分授权、额度或风险信息未能完整核实。不要把缺少的数据理解为不存在风险。</Text></View> : null}
        {result.status === 1 && (result.approvals?.length ?? 0) === 0 ? <View style={styles.empty}><Text style={styles.emptyIcon}>✓</Text><Text style={styles.stateTitle}>本次未发现 USDT 授权</Text><Text style={styles.stateText}>枚举已完成且结果为空，但这不代表地址不存在其他类型风险。</Text></View> : null}
        {(result.approvals ?? []).map((item, index) => {
          const detailURL = safeOfficialURL(item.official_detail_url);
          return <View key={`${item.spender}-${index}`} style={styles.approvalCard}>
            <View style={styles.approvalTop}><View style={styles.approvalMain}><Text style={styles.approvalName}>{item.display_name || '未识别授权对象'}</Text><Text style={styles.approvalAddress}>{truncateAddress(item.spender, 11, 9)}</Text></View><Text style={[styles.allowance, item.unlimited_status === 2 && styles.unlimited]}>{allowanceText(item)}</Text></View>
            <View style={styles.tags}><Text style={styles.tag}>{spenderType(item.spender_type)}</Text>{item.active ? <Text style={styles.activeTag}>当前有效</Text> : <Text style={styles.mutedTag}>非活跃</Text>}{(item.risk_labels ?? []).map((label) => <Text key={label} style={styles.riskTag}>{label}</Text>)}</View>
            <Text style={styles.detail}>额度核实：{item.allowance_verified ? '已核实' : '未核实'} · 风险来源：{item.risk_source || '暂无'}</Text>
            {detailURL ? <Pressable onPress={() => void Linking.openURL(detailURL)}><Text style={styles.link}>在 TRONSCAN 查看公开详情 ›</Text></Pressable> : null}
          </View>;
        })}
        {(result.partial_errors ?? []).length ? <View style={styles.partial}><Text style={styles.partialTitle}>查询提示</Text>{result.partial_errors?.map((item, index) => <Text key={`${item.code}-${index}`} style={styles.partialText}>• {item.message || item.code || '部分数据获取失败'}</Text>)}</View> : null}
      </> : null}

      <Modal visible={showFavorite} transparent animationType="fade"><View style={styles.overlay}><View style={styles.modal}><Text style={styles.modalTitle}>收藏公开地址</Text><Text style={styles.stateText}>{truncateAddress(address, 12, 10)}</Text><TextInput value={note} onChangeText={setNote} maxLength={noteLimit} placeholder="备注，例如：日常钱包" style={styles.noteInput} /><Text style={styles.count}>{note.length}/{noteLimit}</Text>{favoriteError ? <Text style={styles.inlineError}>{favoriteError}</Text> : null}<View style={styles.modalActions}><Pressable onPress={() => setShowFavorite(false)} style={styles.cancel}><Text style={styles.outlineText}>取消</Text></Pressable><Pressable onPress={() => void persistFavorite()} style={styles.save}><Text style={styles.primaryText}>保存</Text></Pressable></View></View></View></Modal>
    </PageShell>
  );
}

const styles = StyleSheet.create({
  back: { marginBottom: 15, color: '#2563eb', fontWeight: '800' },
  stateCard: { padding: 38, borderRadius: 20, alignItems: 'center', backgroundColor: '#fff', borderWidth: 1, borderColor: '#e2e8f0' }, stateIcon: { color: '#2563eb', fontSize: 40 }, stateTitle: { marginTop: 9, color: '#0f172a', fontSize: 18, fontWeight: '900' }, stateText: { marginTop: 7, color: '#64748b', lineHeight: 20, textAlign: 'center' },
  errorCard: { padding: 25, borderRadius: 20, backgroundColor: '#fff1f2', borderWidth: 1, borderColor: '#fecdd3' }, primaryButton: { marginTop: 18, alignSelf: 'flex-start', borderRadius: 10, paddingHorizontal: 20, paddingVertical: 12, backgroundColor: '#2563eb' }, primaryText: { color: '#fff', fontWeight: '800' },
  summary: { padding: 22, borderRadius: 20, backgroundColor: '#fff', borderWidth: 1, borderColor: '#e2e8f0' }, summaryTop: { flexDirection: 'row', justifyContent: 'space-between', gap: 12 }, kicker: { color: '#64748b', fontSize: 12, fontWeight: '700' }, summaryAddress: { marginTop: 5, color: '#0f172a', fontSize: 18, fontWeight: '900' }, badge: { alignSelf: 'flex-start', borderRadius: 99, paddingHorizontal: 12, paddingVertical: 7 }, badgeText: { fontSize: 12, fontWeight: '900' },
  metrics: { marginTop: 22, flexDirection: 'row', flexWrap: 'wrap', gap: 10 }, metric: { minWidth: 130, flexGrow: 1, borderRadius: 13, padding: 14, backgroundColor: '#f8fafc' }, metricValue: { color: '#0f172a', fontSize: 18, fontWeight: '900' }, metricLabel: { marginTop: 4, color: '#64748b', fontSize: 11 }, reason: { marginTop: 15, color: '#475569', lineHeight: 20 }, toolbar: { marginTop: 15, flexDirection: 'row', flexWrap: 'wrap', gap: 8 }, outlineButton: { borderWidth: 1, borderColor: '#bfdbfe', borderRadius: 9, paddingHorizontal: 13, paddingVertical: 9 }, outlineText: { color: '#2563eb', fontSize: 12, fontWeight: '800' }, inlineError: { marginTop: 9, color: '#dc2626', fontSize: 12 },
  warning: { marginTop: 13, padding: 17, borderRadius: 14, backgroundColor: '#fffbeb', borderWidth: 1, borderColor: '#fde68a' }, warningTitle: { color: '#92400e', fontWeight: '900' }, warningText: { marginTop: 5, color: '#a16207', lineHeight: 19, fontSize: 12 }, empty: { marginTop: 13, padding: 28, alignItems: 'center', borderRadius: 16, backgroundColor: '#f0fdf4', borderWidth: 1, borderColor: '#bbf7d0' }, emptyIcon: { color: '#16a34a', fontSize: 30, fontWeight: '900' },
  approvalCard: { marginTop: 12, padding: 18, borderRadius: 16, backgroundColor: '#fff', borderWidth: 1, borderColor: '#e2e8f0' }, approvalTop: { flexDirection: 'row', justifyContent: 'space-between', gap: 12 }, approvalMain: { flex: 1 }, approvalName: { color: '#0f172a', fontWeight: '900' }, approvalAddress: { marginTop: 4, color: '#64748b', fontSize: 12 }, allowance: { color: '#0f172a', fontWeight: '900' }, unlimited: { color: '#dc2626' }, tags: { marginTop: 13, flexDirection: 'row', flexWrap: 'wrap', gap: 6 }, tag: { color: '#334155', backgroundColor: '#f1f5f9', borderRadius: 99, paddingHorizontal: 9, paddingVertical: 5, fontSize: 11 }, activeTag: { color: '#15803d', backgroundColor: '#dcfce7', borderRadius: 99, paddingHorizontal: 9, paddingVertical: 5, fontSize: 11 }, mutedTag: { color: '#64748b', backgroundColor: '#f1f5f9', borderRadius: 99, paddingHorizontal: 9, paddingVertical: 5, fontSize: 11 }, riskTag: { color: '#b91c1c', backgroundColor: '#fee2e2', borderRadius: 99, paddingHorizontal: 9, paddingVertical: 5, fontSize: 11 }, detail: { marginTop: 12, color: '#64748b', fontSize: 11 }, link: { marginTop: 12, color: '#2563eb', fontSize: 12, fontWeight: '800' },
  partial: { marginTop: 12, padding: 17, borderRadius: 14, backgroundColor: '#f8fafc' }, partialTitle: { color: '#334155', fontWeight: '900' }, partialText: { marginTop: 6, color: '#64748b', fontSize: 12 },
  overlay: { flex: 1, padding: 20, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(15,23,42,0.5)' }, modal: { width: '100%', maxWidth: 450, padding: 23, borderRadius: 18, backgroundColor: '#fff' }, modalTitle: { color: '#0f172a', fontSize: 20, fontWeight: '900' }, noteInput: { marginTop: 15, padding: 12, borderWidth: 1, borderColor: '#cbd5e1', borderRadius: 10 }, count: { marginTop: 5, textAlign: 'right', color: '#94a3b8', fontSize: 11 }, modalActions: { marginTop: 17, flexDirection: 'row', justifyContent: 'flex-end', gap: 9 }, cancel: { paddingHorizontal: 18, paddingVertical: 12, borderWidth: 1, borderColor: '#cbd5e1', borderRadius: 10 }, save: { paddingHorizontal: 22, paddingVertical: 12, backgroundColor: '#2563eb', borderRadius: 10 },
});
