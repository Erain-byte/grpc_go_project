import { environment } from '@/config/environment';
import type { ApprovalResult } from '@/types/approval';
type APIError = { message?: string; error?: string };

export async function checkTronApprovals(address: string, signal?: AbortSignal): Promise<ApprovalResult> {
  const response = await fetch(`${environment.gatewayURL}/v1/tron/usdt/approvals/check`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ address }), signal,
  });
  const payload = (await response.json().catch(() => ({}))) as ApprovalResult & APIError;
  if (!response.ok) throw new Error(payload.message ?? payload.error ?? `查询失败（HTTP ${response.status}）`);
  return payload;
}
