package middleware

import (
	"admin/internal/auth"
	"admin/pkg/apperorr"
	"context"
	"fmt"
	"strings"

	commonv1 "github.com/Erain-byte/grpc_go_project/proto/common/v1"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	metadataUserID    = "x-user-id"
	metadataUserRole  = "x-user-role"
	metadataSessionID = "x-session-id"
	metadataTokenID   = "x-token-id"
)

// AuthInterceptor 校验可信 Gateway 通过 gRPC metadata 传递的用户身份。
// 这些 metadata 只有在 Gateway 与内部服务之间启用 mTLS（或同等网络隔离）时才可信。
type AuthInterceptor struct{}

func NewAuthInterceptor() *AuthInterceptor {
	return &AuthInterceptor{}
}

func (m *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Health 服务来自 gRPC 官方 Proto，无法添加项目自定义的 auth Option。
		// Consul 会调用该方法判断实例是否健康，因此这里明确放行。
		if info.FullMethod == healthpb.Health_Check_FullMethodName {
			return handler(ctx, req)
		}

		rule, err := authRuleForMethod(info.FullMethod)
		if err != nil {
			return nil, err
		}
		if rule.GetPublic() {
			return handler(ctx, req)
		}

		authInfo, err := authInfoFromMetadata(ctx)
		if err != nil {
			return nil, err
		}
		if !roleAllowed(authInfo.Role, rule.GetRoles()) {
			return nil, apperorr.Forbidden("the current role cannot access this method")
		}

		return handler(auth.NewContext(ctx, authInfo), req)
	}
}

// authRuleForMethod 从编译后的 Proto 方法描述符中读取自定义 auth Option。
// 未配置 auth Option 的业务方法默认需要登录，遵循 fail closed 原则。
func authRuleForMethod(fullMethod string) (*commonv1.AuthRule, error) {
	descriptorName, err := grpcMethodDescriptorName(fullMethod)
	if err != nil {
		return nil, err
	}
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(descriptorName)
	if err != nil {
		return nil, fmt.Errorf("find protobuf method descriptor %q: %w", descriptorName, err)
	}
	method, ok := descriptor.(protoreflect.MethodDescriptor)
	if !ok {
		return nil, fmt.Errorf("protobuf descriptor %q is not a method", descriptorName)
	}
	options, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok {
		return nil, fmt.Errorf("protobuf method %q has invalid options", descriptorName)
	}
	if !proto.HasExtension(options, commonv1.E_Auth) {
		return &commonv1.AuthRule{}, nil
	}
	rule, ok := proto.GetExtension(options, commonv1.E_Auth).(*commonv1.AuthRule)
	if !ok || rule == nil {
		return nil, fmt.Errorf("protobuf method %q has an invalid auth option", descriptorName)
	}
	return rule, nil
}

func grpcMethodDescriptorName(fullMethod string) (protoreflect.FullName, error) {
	parts := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid gRPC full method name %q", fullMethod)
	}
	return protoreflect.FullName(parts[0] + "." + parts[1]), nil
}

func authInfoFromMetadata(ctx context.Context) (auth.AuthInfo, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return auth.AuthInfo{}, apperorr.Unauthorized("caller identity metadata is missing")
	}
	adminID, err := requiredMetadataValue(md, metadataUserID)
	if err != nil {
		return auth.AuthInfo{}, err
	}
	role, err := requiredMetadataValue(md, metadataUserRole)
	if err != nil {
		return auth.AuthInfo{}, err
	}
	return auth.AuthInfo{
		AdminID:   adminID,
		Role:      role,
		SessionID: firstMetadataValue(md, metadataSessionID),
		TokenID:   firstMetadataValue(md, metadataTokenID),
	}, nil
}

func requiredMetadataValue(md metadata.MD, key string) (string, error) {
	value := firstMetadataValue(md, key)
	if value == "" {
		return "", apperorr.Unauthorized(key + " metadata is missing")
	}
	return value, nil
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

// roles 为空表示任意已登录角色；非空时必须精确匹配其中一个角色。
func roleAllowed(role string, roles []string) bool {
	if len(roles) == 0 {
		return true
	}
	for _, allowedRole := range roles {
		if role == allowedRole {
			return true
		}
	}
	return false
}
