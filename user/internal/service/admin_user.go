package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"user/internal/model"
	tracing "user/internal/tracer"
	"user/pkg/apperror"

	userv1 "github.com/Erain-byte/grpc_go_project/proto/user/v1"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const (
	defaultUserListPageSize int32 = 20
	maxUserListPageSize     int32 = 100
)

// GetUserInfo serves two intentional cases:
//   - user: request.UserId must be empty and the caller reads their own profile;
//   - admin/super-admin: request.UserId selects the target local user.
func (s *UserService) GetUserInfo(
	ctx context.Context,
	req *userv1.GetUserInfoRequest,
) (response *userv1.GetUserInfoResponse, returnErr error) {
	ctx, span := s.tracer.Start(ctx, "user.get_user_info", trace.WithSpanKind(trace.SpanKindInternal))
	defer func() {
		tracing.RecordError(span, returnErr, "get user information failed")
		span.End()
	}()

	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}

	targetID := ""
	if req != nil {
		targetID = strings.TrimSpace(req.GetUserId())
	}

	var user *model.User
	if targetID == "" {
		if isAdminRole(identity.Role) {
			return nil, apperror.InvalidArgument("user_id is required for an administrator query")
		}
		user, err = s.repository.FindBySubject(ctx, identity.Subject)
	} else {
		if !isAdminRole(identity.Role) {
			return nil, apperror.Forbidden("only administrators may query another user")
		}
		userID, parseErr := strconv.ParseInt(targetID, 10, 64)
		if parseErr != nil || userID <= 0 {
			return nil, apperror.InvalidArgument("user_id must be a positive integer")
		}
		user, err = s.repository.FindByID(ctx, userID)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.NotFound("local user was not found")
	}
	if err != nil {
		return nil, err
	}
	return &userv1.GetUserInfoResponse{Success: true, UserInfo: toProtoUser(user)}, nil
}

// GetUserList is an administrator-only cross-shard user list.
func (s *UserService) GetUserList(
	ctx context.Context,
	req *userv1.GetUserListRequest,
) (response *userv1.GetUserListResponse, returnErr error) {
	ctx, span := s.tracer.Start(ctx, "user.get_user_list", trace.WithSpanKind(trace.SpanKindInternal))
	defer func() {
		tracing.RecordError(span, returnErr, "get user list failed")
		span.End()
	}()

	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if !isAdminRole(identity.Role) {
		return nil, apperror.Forbidden("only administrators may list users")
	}

	page, pageSize, err := normalizeUserPage(req)
	if err != nil {
		return nil, err
	}
	offset := int((page - 1) * pageSize)
	users, total, err := s.repository.List(ctx, offset, int(pageSize))
	if err != nil {
		return nil, err
	}

	result := make([]*userv1.UserInfo, 0, len(users))
	for _, user := range users {
		result = append(result, toProtoUser(user))
	}
	return &userv1.GetUserListResponse{
		Success:  true,
		UserList: result,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func normalizeUserPage(req *userv1.GetUserListRequest) (int32, int32, error) {
	if req == nil {
		return 1, defaultUserListPageSize, nil
	}
	page, pageSize := req.GetPage(), req.GetPageSize()
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = defaultUserListPageSize
	}
	if page < 1 || pageSize < 1 || pageSize > maxUserListPageSize {
		return 0, 0, apperror.InvalidArgument("page must be positive and page_size must be between 1 and 100")
	}
	return page, pageSize, nil
}

func isAdminRole(role string) bool {
	return role == "admin" || role == "super-admin"
}
