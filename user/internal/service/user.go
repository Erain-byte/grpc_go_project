package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"user/internal/auth"
	"user/internal/model"
	"user/internal/repository"
	"user/internal/svc"
	tracing "user/internal/tracer"
	"user/pkg/apperror"

	userv1 "github.com/Erain-byte/grpc_go_project/proto/user/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type UserService struct {
	repository *repository.UserRepository
	ids        interface{ NextID() (int64, error) }
	tracer     trace.Tracer
}

func NewUserService(svcCtx *svc.ServiceContext) *UserService {
	return &UserService{
		repository: repository.NewUserRepository(svcCtx.Database.Gorm(), svcCtx.Router),
		ids:        svcCtx.IDs,
		tracer:     otel.Tracer("user/internal/service"),
	}
}

func (s *UserService) InitializeUser(
	ctx context.Context,
	_ *userv1.InitializeUserRequest,
) (response *userv1.InitializeUserResponse, returnErr error) {
	ctx, span := s.tracer.Start(
		ctx,
		"user.initialize_user",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer func() {
		tracing.RecordError(span, returnErr, "initialize user failed")
		span.End()
	}()

	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := s.ids.NextID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	user, created, err := s.repository.FindOrCreateBySubject(ctx, &model.User{
		ID: id, KeycloakSubject: identity.Subject, Username: identity.Username,
		Email: optionalString(identity.Email), Nickname: identity.Username,
		Status: model.UserStatusActive, LastLoginAt: &now,
	})
	if err != nil {
		return nil, err
	}
	return &userv1.InitializeUserResponse{UserInfo: toProtoUser(user), NewlyCreated: created}, nil
}

func (s *UserService) GetUserInfo(
	ctx context.Context,
	_ *userv1.GetUserInfoRequest,
) (response *userv1.GetUserInfoResponse, returnErr error) {
	ctx, span := s.tracer.Start(
		ctx,
		"user.get_user_info",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer func() {
		tracing.RecordError(span, returnErr, "get user information failed")
		span.End()
	}()

	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.repository.FindBySubject(ctx, identity.Subject)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.NotFound("local user has not been initialized")
	}
	if err != nil {
		return nil, err
	}
	return &userv1.GetUserInfoResponse{Success: true, UserInfo: toProtoUser(user)}, nil
}

func authenticatedIdentity(ctx context.Context) (auth.Identity, error) {
	identity, ok := auth.FromContext(ctx)
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		return auth.Identity{}, apperror.Unauthorized("authenticated identity is missing")
	}
	return identity, nil
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func toProtoUser(user *model.User) *userv1.UserInfo {
	if user == nil {
		return nil
	}
	result := &userv1.UserInfo{Id: strconv.FormatInt(user.ID, 10), Username: user.Username, Nickname: user.Nickname,
		AvatarUrl: user.AvatarURL, Status: userv1.UserStatus(user.Status), CreatedAt: timestamppb.New(user.CreatedAt), UpdatedAt: timestamppb.New(user.UpdatedAt)}
	if user.Email != nil {
		result.Email = *user.Email
	}
	if user.Phone != nil {
		result.Phone = *user.Phone
	}
	if user.LastLoginAt != nil {
		result.LastLoginAt = timestamppb.New(*user.LastLoginAt)
	}
	return result
}
