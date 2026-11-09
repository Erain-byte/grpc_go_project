export type ProtoTimestamp = {
  seconds?: number | string;
  nanos?: number;
};

export type ApprovalStatus = 0 | 1 | 2 | 3;
export type SpenderType = 0 | 1 | 2 | 3;
export type UnlimitedStatus = 0 | 1 | 2 | 3 | 4;

export type TronApproval = {
  spender: string;
  spender_type?: SpenderType;
  display_name?: string;
  raw_allowance?: string;
  decimals?: number;
  display_allowance?: string;
  unlimited_status?: UnlimitedStatus;
  allowance_verified_at?: ProtoTimestamp;
  risk_labels?: string[];
  risk_source?: string;
  risk_checked_at?: ProtoTimestamp;
  allowance_verified?: boolean;
  active?: boolean;
  official_detail_url?: string;
};

export type ApprovalPartialError = {
  code?: string;
  message?: string;
  retryable?: boolean;
  spender?: string;
};

export type ApprovalResult = {
  request_id?: string;
  network?: string;
  address: string;
  token_contract?: string;
  status: ApprovalStatus;
  enumeration_complete?: boolean;
  allowance_verification_complete?: boolean;
  risk_data_available?: boolean;
  truncated?: boolean;
  reason?: string;
  is_cached?: boolean;
  queried_at?: ProtoTimestamp;
  approvals?: TronApproval[];
  partial_errors?: ApprovalPartialError[];
};

export type FavoriteAddress = {
  network: 'tron';
  address: string;
  note: string;
  updatedAt: string;
  lastResult?: ApprovalResult;
};
