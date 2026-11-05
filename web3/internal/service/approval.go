package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"web3/internal/config"
	"web3/internal/repository"
	"web3/internal/tron"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const tronMainnetName = "TRON_MAINNET"

var maxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

type approvalUpstream interface {
	Enabled() bool
	USDTContract() string
	ListApprovals(context.Context, string, int, int) (*tron.ApprovalPage, error)
	RiskySpenders(context.Context, string) (map[string]struct{}, error)
	Allowance(context.Context, string, string) (*big.Int, error)
	USDTDecimals(context.Context, string) (uint32, error)
}

type approvalCache interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, any, time.Duration) error
}

type approvalUsage interface {
	ReserveQuery(context.Context, time.Time, string, int64, int64) error
	ReserveUpstreamCall(context.Context, time.Time, int64) error
	RecordResult(context.Context, time.Time, string, bool) error
}

// TronApprovalService orchestrates address validation, TRONSCAN pagination,
// current on-chain allowance verification, risk labels and short Redis caching.
type TronApprovalService struct {
	upstream           approvalUpstream
	cache              approvalCache
	usage              approvalUsage
	logger             *zap.Logger
	usageHMACKey       []byte
	dailyQueryLimit    int64
	dailyAddressLimit  int64
	dailyUpstreamLimit int64
	cacheTTL           time.Duration
	pageSize           int
	maxPages           int
	maxSpenders        int
	verifyWorkers      int
	now                func() time.Time
	inflightMu         sync.Mutex
	inflight           map[string]*approvalCall
}

type approvalCall struct {
	done     chan struct{}
	response *web3v1.CheckTronUSDTApprovalsResponse
	err      error
}

func NewTronApprovalService(upstream approvalUpstream, cache approvalCache, usage approvalUsage, logger *zap.Logger, cfg config.TronApprovalConfig) (*TronApprovalService, error) {
	if upstream == nil || cache == nil || usage == nil || logger == nil {
		return nil, apperror.InvalidArgument("TRON approval service dependencies are incomplete")
	}
	cacheTTL, err := time.ParseDuration(cfg.CacheTTL)
	if err != nil || cacheTTL <= 0 {
		return nil, apperror.InvalidArgument("TRON approval cache TTL is invalid")
	}
	if cfg.PageSize < 1 || cfg.PageSize > 50 || cfg.MaxPages < 1 || cfg.MaxSpenders < 1 || cfg.VerifyWorkers < 1 {
		return nil, apperror.InvalidArgument("TRON approval query limits are invalid")
	}
	if cfg.Enabled && (len(cfg.UsageHMACKey) < 32 || cfg.DailyQueryLimit <= 0 || cfg.DailyAddressLimit <= 0 || cfg.DailyUpstreamCallLimit <= 0) {
		return nil, apperror.InvalidArgument("TRON approval usage budget configuration is invalid")
	}
	return &TronApprovalService{
		upstream:           upstream,
		cache:              cache,
		usage:              usage,
		logger:             logger,
		usageHMACKey:       []byte(cfg.UsageHMACKey),
		dailyQueryLimit:    cfg.DailyQueryLimit,
		dailyAddressLimit:  cfg.DailyAddressLimit,
		dailyUpstreamLimit: cfg.DailyUpstreamCallLimit,
		cacheTTL:           cacheTTL,
		pageSize:           cfg.PageSize,
		maxPages:           cfg.MaxPages,
		maxSpenders:        cfg.MaxSpenders,
		verifyWorkers:      cfg.VerifyWorkers,
		now:                time.Now,
		inflight:           make(map[string]*approvalCall),
	}, nil
}

func (s *TronApprovalService) CheckTronUSDTApprovals(ctx context.Context, req *web3v1.CheckTronUSDTApprovalsRequest) (*web3v1.CheckTronUSDTApprovalsResponse, error) {
	if req == nil {
		return nil, apperror.InvalidArgument("request is nil")
	}
	address := strings.TrimSpace(req.GetAddress())
	if _, err := tron.DecodeMainnetAddress(address); err != nil {
		return nil, apperror.InvalidArgument("invalid TRON Base58Check address")
	}
	// 功能关闭时不要生成 HMAC 或写入用量表；这也允许开发环境在未配置密钥时启动。
	if !s.upstream.Enabled() {
		return nil, apperror.Unavailable("TRON approval query is not enabled")
	}
	queryTime := s.now().UTC()
	addressHMAC := s.addressHMAC(address)
	if err := s.usage.ReserveQuery(ctx, queryTime, addressHMAC, s.dailyQueryLimit, s.dailyAddressLimit); err != nil {
		s.logBudgetError("reserve query budget", err, zap.String("address_hmac", addressHMAC))
		return nil, usageError(err)
	}
	if cached := s.loadCache(ctx, address); cached != nil {
		cached.RequestId = uuid.NewString()
		cached.IsCached = true
		_ = s.usage.RecordResult(ctx, queryTime, approvalStatusName(cached.Status), true)
		return cached, nil
	}

	s.inflightMu.Lock()
	if active := s.inflight[address]; active != nil {
		s.inflightMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-active.done:
			if active.response == nil {
				_ = s.usage.RecordResult(ctx, queryTime, "failed", false)
				return nil, active.err
			}
			response := proto.Clone(active.response).(*web3v1.CheckTronUSDTApprovalsResponse)
			response.RequestId = uuid.NewString()
			_ = s.usage.RecordResult(ctx, queryTime, approvalStatusName(response.Status), false)
			return response, active.err
		}
	}
	call := &approvalCall{done: make(chan struct{})}
	s.inflight[address] = call
	s.inflightMu.Unlock()

	response, err := s.check(ctx, address)
	s.inflightMu.Lock()
	call.response, call.err = response, err
	delete(s.inflight, address)
	close(call.done)
	s.inflightMu.Unlock()
	if response == nil {
		_ = s.usage.RecordResult(ctx, queryTime, "failed", false)
	} else {
		_ = s.usage.RecordResult(ctx, queryTime, approvalStatusName(response.Status), false)
	}
	return response, err
}

func (s *TronApprovalService) check(ctx context.Context, address string) (*web3v1.CheckTronUSDTApprovalsResponse, error) {
	if !s.upstream.Enabled() {
		return nil, apperror.Unavailable("TRON approval query is not enabled")
	}

	queriedAt := s.now().UTC()
	response := &web3v1.CheckTronUSDTApprovalsResponse{
		RequestId:     uuid.NewString(),
		Network:       tronMainnetName,
		Address:       address,
		TokenContract: s.upstream.USDTContract(),
		QueriedAt:     timestamppb.New(queriedAt),
		Approvals:     []*web3v1.TronUSDTApproval{},
		PartialErrors: []*web3v1.TronApprovalPartialError{},
	}

	candidates, addressTypes, names, enumerationComplete, truncated, enumerateErrors := s.enumerate(ctx, address)
	response.EnumerationComplete = enumerationComplete
	response.Truncated = truncated
	response.PartialErrors = append(response.PartialErrors, enumerateErrors...)
	if len(candidates) == 0 && !enumerationComplete {
		response.Status = web3v1.TronApprovalCheckStatus_TRON_APPROVAL_CHECK_STATUS_FAILED
		response.Reason = "未能完成 TRONSCAN 授权对象查询，请稍后重试"
		return response, nil
	}

	riskBudgetErr := s.reserveUpstream(ctx)
	var riskySpenders map[string]struct{}
	var riskErr error
	if riskBudgetErr != nil {
		riskErr = riskBudgetErr
	} else {
		riskySpenders, riskErr = s.upstream.RiskySpenders(ctx, address)
	}
	response.RiskDataAvailable = riskErr == nil
	if riskErr != nil {
		response.PartialErrors = append(response.PartialErrors, partialError(riskErr, "RISK_DATA_UNAVAILABLE", "风险标签暂时不可用", ""))
	}
	decimalsBudgetErr := s.reserveUpstream(ctx)
	var decimals uint32
	var decimalsErr error
	if decimalsBudgetErr != nil {
		decimalsErr = decimalsBudgetErr
	} else {
		decimals, decimalsErr = s.upstream.USDTDecimals(ctx, address)
	}
	if decimalsErr != nil {
		// 官方 TRON USDT 当前精度为 6；读回失败时仅作为降级展示，并明确返回 partial。
		decimals = 6
		response.PartialErrors = append(response.PartialErrors, partialError(decimalsErr, "TOKEN_DECIMALS_UNAVAILABLE", "USDT 精度未能从合约读回", ""))
	}

	var allowanceComplete bool
	response.Approvals, allowanceComplete = s.verifyAllowances(
		ctx, address, candidates, addressTypes, names, riskySpenders, response.RiskDataAvailable, decimals, queriedAt,
	)
	response.AllowanceVerificationComplete = allowanceComplete && decimalsErr == nil
	for _, approval := range response.Approvals {
		if !approval.GetAllowanceVerified() {
			response.PartialErrors = append(response.PartialErrors, &web3v1.TronApprovalPartialError{
				Code: "ALLOWANCE_UNAVAILABLE", Message: "当前授权额度未核实", Retryable: true, Spender: approval.GetSpender(),
			})
		}
	}

	if response.EnumerationComplete && response.AllowanceVerificationComplete {
		response.Status = web3v1.TronApprovalCheckStatus_TRON_APPROVAL_CHECK_STATUS_SUCCESS
		response.Reason = "本次数据源返回范围已查询完成；未检查账户控制权限及其他资产"
	} else {
		response.Status = web3v1.TronApprovalCheckStatus_TRON_APPROVAL_CHECK_STATUS_PARTIAL
		response.Reason = "结果未完整获取；未检查账户控制权限及其他资产"
	}
	s.storeCache(ctx, address, response)
	return response, nil
}

func (s *TronApprovalService) enumerate(ctx context.Context, owner string) (
	[]tron.ApprovalRecord,
	map[string]web3v1.TronSpenderType,
	map[string]string,
	bool,
	bool,
	[]*web3v1.TronApprovalPartialError,
) {
	bySpender := make(map[string]tron.ApprovalRecord)
	addressTypes := make(map[string]web3v1.TronSpenderType)
	names := make(map[string]string)
	errorsList := make([]*web3v1.TronApprovalPartialError, 0)
	start := 0
	complete := false
	truncated := false

	for pageNumber := 0; pageNumber < s.maxPages; pageNumber++ {
		if err := s.reserveUpstream(ctx); err != nil {
			errorsList = append(errorsList, partialError(err, "UPSTREAM_BUDGET_EXHAUSTED", "上游调用预算已用尽", ""))
			break
		}
		page, err := s.upstream.ListApprovals(ctx, owner, start, s.pageSize)
		if err != nil {
			errorsList = append(errorsList, partialError(err, "APPROVAL_ENUMERATION_FAILED", "授权列表分页查询失败", ""))
			break
		}
		for address, isContract := range page.ContractMap {
			if isContract {
				addressTypes[address] = web3v1.TronSpenderType_TRON_SPENDER_TYPE_CONTRACT
			} else {
				addressTypes[address] = web3v1.TronSpenderType_TRON_SPENDER_TYPE_ADDRESS
			}
		}
		for address, info := range page.ContractInfo {
			names[address] = firstNonEmpty(info.Tag1, info.PublicTag, info.Name)
		}
		for address, info := range page.NormalAddressInfo {
			names[address] = firstNonEmpty(info.PublicTag, info.Tag1, info.Name)
		}
		for _, approval := range page.Data {
			if approval.ContractAddress != s.upstream.USDTContract() || approval.FromAddress != owner {
				continue
			}
			spender := strings.TrimSpace(approval.ToAddress)
			if spender == "" || spender == s.upstream.USDTContract() {
				continue
			}
			if _, exists := bySpender[spender]; exists {
				continue
			}
			if len(bySpender) >= s.maxSpenders {
				truncated = true
				continue
			}
			bySpender[spender] = approval
			if names[spender] == "" {
				names[spender] = approval.Project.Name
			}
		}

		count := len(page.Data)
		start += count
		if count == 0 || start >= page.Total {
			complete = !truncated
			break
		}
		if pageNumber == s.maxPages-1 {
			truncated = true
		}
	}
	if truncated {
		errorsList = append(errorsList, &web3v1.TronApprovalPartialError{
			Code: "SCAN_LIMIT_REACHED", Message: "已达到服务端授权扫描上限", Retryable: false,
		})
		complete = false
	}
	candidates := make([]tron.ApprovalRecord, 0, len(bySpender))
	for _, approval := range bySpender {
		candidates = append(candidates, approval)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ToAddress < candidates[j].ToAddress })
	return candidates, addressTypes, names, complete, truncated, errorsList
}

func (s *TronApprovalService) verifyAllowances(
	ctx context.Context,
	owner string,
	candidates []tron.ApprovalRecord,
	addressTypes map[string]web3v1.TronSpenderType,
	names map[string]string,
	risky map[string]struct{},
	riskAvailable bool,
	decimals uint32,
	checkedAt time.Time,
) ([]*web3v1.TronUSDTApproval, bool) {
	approvals := make([]*web3v1.TronUSDTApproval, len(candidates))
	jobs := make(chan int)
	workers := s.verifyWorkers
	if workers > len(candidates) {
		workers = len(candidates)
	}
	var wait sync.WaitGroup
	wait.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wait.Done()
			for index := range jobs {
				candidate := candidates[index]
				approvals[index] = s.verifyOne(ctx, owner, candidate, addressTypes, names, risky, riskAvailable, decimals, checkedAt)
			}
		}()
	}
	for index := range candidates {
		jobs <- index
	}
	close(jobs)
	wait.Wait()

	complete := true
	for _, approval := range approvals {
		if approval == nil || !approval.AllowanceVerified {
			complete = false
		}
	}
	sort.SliceStable(approvals, func(i, j int) bool {
		iRisk, jRisk := len(approvals[i].RiskLabels) > 0, len(approvals[j].RiskLabels) > 0
		if iRisk != jRisk {
			return iRisk
		}
		return approvals[i].Spender < approvals[j].Spender
	})
	return approvals, complete
}

func (s *TronApprovalService) verifyOne(
	ctx context.Context,
	owner string,
	candidate tron.ApprovalRecord,
	addressTypes map[string]web3v1.TronSpenderType,
	names map[string]string,
	risky map[string]struct{},
	riskAvailable bool,
	decimals uint32,
	checkedAt time.Time,
) *web3v1.TronUSDTApproval {
	spender := candidate.ToAddress
	spenderType := addressTypes[spender]
	if spenderType == web3v1.TronSpenderType_TRON_SPENDER_TYPE_UNSPECIFIED {
		spenderType = web3v1.TronSpenderType_TRON_SPENDER_TYPE_UNKNOWN
	}
	result := &web3v1.TronUSDTApproval{
		Spender:           spender,
		SpenderType:       spenderType,
		DisplayName:       firstNonEmpty(names[spender], "未识别"),
		Decimals:          decimals,
		UnlimitedStatus:   web3v1.TronUnlimitedStatus_TRON_UNLIMITED_STATUS_UNKNOWN,
		OfficialDetailUrl: "https://tronscan.org/#/address/" + spender,
	}
	if riskAvailable {
		result.RiskSource = "TRONSCAN"
		result.RiskCheckedAt = timestamppb.New(checkedAt)
		if _, found := risky[spender]; found {
			result.RiskLabels = []string{"TRONSCAN_KNOWN_RISK"}
		}
	}

	if err := s.reserveUpstream(ctx); err != nil {
		return result
	}
	allowance, err := s.upstream.Allowance(ctx, owner, spender)
	if err != nil {
		return result
	}
	result.RawAllowance = allowance.String()
	result.DisplayAllowance = formatUnits(allowance, result.Decimals)
	result.AllowanceVerified = true
	result.AllowanceVerifiedAt = timestamppb.New(checkedAt)
	result.Active = allowance.Sign() > 0
	isMaximum := allowance.Cmp(maxUint256) == 0
	if isMaximum != candidate.Unlimited {
		result.UnlimitedStatus = web3v1.TronUnlimitedStatus_TRON_UNLIMITED_STATUS_CONFLICT
	} else if isMaximum {
		result.UnlimitedStatus = web3v1.TronUnlimitedStatus_TRON_UNLIMITED_STATUS_UNLIMITED
	} else {
		result.UnlimitedStatus = web3v1.TronUnlimitedStatus_TRON_UNLIMITED_STATUS_FINITE
	}
	return result
}

func (s *TronApprovalService) loadCache(ctx context.Context, address string) *web3v1.CheckTronUSDTApprovalsResponse {
	payload, err := s.cache.Get(ctx, approvalCacheKey(address))
	if err != nil || payload == "" {
		return nil
	}
	response := new(web3v1.CheckTronUSDTApprovalsResponse)
	if protojson.Unmarshal([]byte(payload), response) != nil {
		return nil
	}
	return response
}

func (s *TronApprovalService) storeCache(ctx context.Context, address string, response *web3v1.CheckTronUSDTApprovalsResponse) {
	payload, err := protojson.Marshal(response)
	if err != nil {
		return
	}
	_ = s.cache.Set(ctx, approvalCacheKey(address), payload, s.cacheTTL)
}

func approvalCacheKey(address string) string {
	digest := sha256.Sum256([]byte(address))
	return "tron:usdt:approvals:" + hex.EncodeToString(digest[:])
}

func (s *TronApprovalService) addressHMAC(address string) string {
	mac := hmac.New(sha256.New, s.usageHMACKey)
	_, _ = mac.Write([]byte(address))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *TronApprovalService) reserveUpstream(ctx context.Context) error {
	err := s.usage.ReserveUpstreamCall(ctx, s.now().UTC(), s.dailyUpstreamLimit)
	if err != nil {
		s.logBudgetError("reserve upstream budget", err)
	}
	return err
}

func (s *TronApprovalService) logBudgetError(operation string, err error, fields ...zap.Field) {
	fields = append(fields, zap.String("operation", operation), zap.Error(err))
	if errors.Is(err, repository.ErrDailyQueryLimit) ||
		errors.Is(err, repository.ErrDailyAddressLimit) ||
		errors.Is(err, repository.ErrDailyUpstreamLimit) {
		s.logger.Warn("TRON approval daily budget exhausted", fields...)
		return
	}
	s.logger.Error("TRON approval usage storage failed", fields...)
}

func usageError(err error) error {
	if errors.Is(err, repository.ErrDailyQueryLimit) ||
		errors.Is(err, repository.ErrDailyAddressLimit) ||
		errors.Is(err, repository.ErrDailyUpstreamLimit) {
		return apperror.TooManyRequests("TRON daily query budget is exhausted")
	}
	return apperror.Unavailable("TRON usage budget storage is unavailable")
}

func approvalStatusName(status web3v1.TronApprovalCheckStatus) string {
	switch status {
	case web3v1.TronApprovalCheckStatus_TRON_APPROVAL_CHECK_STATUS_SUCCESS:
		return "success"
	case web3v1.TronApprovalCheckStatus_TRON_APPROVAL_CHECK_STATUS_PARTIAL:
		return "partial"
	default:
		return "failed"
	}
}

func partialError(err error, code, message, spender string) *web3v1.TronApprovalPartialError {
	retryable := !errors.Is(err, context.Canceled)
	if errors.Is(err, context.DeadlineExceeded) {
		code = "UPSTREAM_TIMEOUT"
	}
	return &web3v1.TronApprovalPartialError{Code: code, Message: message, Retryable: retryable, Spender: spender}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
