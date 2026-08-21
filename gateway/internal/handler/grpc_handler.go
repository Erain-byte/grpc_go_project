package handler

import (
	"context"
	"errors"
	clien "gateway/internal/grpc"
	"gateway/internal/middleware"
	"io"
	"net/http"

	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// 本文件定义 HTTP 请求到 gRPC 请求的转换逻辑。
// GrpcHandlerFunc 保存具体的 gRPC 方法；请求和响应必须是 Handle 的局部变量，
// 避免多个 HTTP 请求并发执行时共享同一份数据。
// 常规接口数据处理
type GrpcHandlerFunc[Req any, Resp any] struct {
	call func(ctx context.Context, req *Req) (*Resp, error)
}

func NewGrpcHandler[Req any, Resp any](
	call func(ctx context.Context, req *Req) (*Resp, error),
) *GrpcHandlerFunc[Req, Resp] {
	return &GrpcHandlerFunc[Req, Resp]{call: call}
}

func (h *GrpcHandlerFunc[Req, Resp]) Handle(c *gin.Context) {
	req := new(Req)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	grpcCtx := WithMetadata(c)
	// 2. 调用 gRPC 方法
	resp, err := h.call(grpcCtx, req)
	if err != nil {
		writeGrpcError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// 流式处理
type LlmHTTPHandler struct {
	clientManager *clien.ClientManager
}

func NewLlmHTTPHandler(clientManager *clien.ClientManager) *LlmHTTPHandler {
	return &LlmHTTPHandler{
		clientManager: clientManager,
	}
}

func (h *LlmHTTPHandler) StreamChat(c *gin.Context) {
	//HTTP JSON → protobuf Request
	var req pbLlm.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//调用gRPC方法
	client, err := h.clientManager.LlmClient(c.Request.Context())
	if err != nil {
		writeGrpcError(c, err)
		return
	}
	grpcCtx := WithMetadata(c)
	stream, err := client.StreamChat(
		grpcCtx,
		&req,
	)
	if err != nil {
		writeGrpcError(c, err)
		return
	}
	// 4. 设置 HTTP SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// 持续接收 gRPC 流
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			c.SSEvent("error", gin.H{"message": err.Error()})
			c.Writer.Flush()
			return
		}
		// 5. 发送数据到 HTTP 客户端
		c.SSEvent("message", resp)
		c.Writer.Flush()
	}
	c.SSEvent("done", gin.H{"completed": true})
	c.Writer.Flush()
}

// 图片上传处理
type UploadedFile struct {
	Filename    string // 文件名
	ContentType string // 文件类型
	Data        []byte // 文件内容
}
type UploadedFileHandler[Req any, Resp any] struct {
	call         func(ctx context.Context, req *Req) (*Resp, error)
	field        string
	maxSize      int64
	buildRequest func(file *UploadedFile) *Req
}

func NewUploadedFileHandler[Req any, Resp any](
	call func(ctx context.Context, req *Req) (*Resp, error),
	field string,
	maxSize int64,
	buildRequest func(file *UploadedFile) *Req,
) *UploadedFileHandler[Req, Resp] {
	return &UploadedFileHandler[Req, Resp]{
		call:         call,
		field:        field,
		maxSize:      maxSize,
		buildRequest: buildRequest,
	}
}
func (h *UploadedFileHandler[Req, Resp]) Handle(c *gin.Context) {
	//限制整个HTTP请求大小
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxSize)
	//从multipart/form-data中解析文件
	// 如果文件大小超过限制，ShouldBind方法会返回错误
	fileHeader, err := c.FormFile(h.field)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//读取文件内容
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()
	//读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uploadedFile := &UploadedFile{
		Filename:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Data:        data,
	}
	grpcCtx := WithMetadata(c)
	//构建请求
	req := h.buildRequest(uploadedFile)
	resp, err := h.call(grpcCtx, req)
	if err != nil {
		writeGrpcError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// 获取用户信息处理
func WithMetadata(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	userID := c.GetString(middleware.ContextUserID)
	role := c.GetString(middleware.ContextRole)
	sessionID := c.GetString(middleware.ContextSessionID)
	if userID != "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(
		ctx,
		"x-user-id", userID,
		"x-user-role", role,
		"x-session-id", sessionID,
	)
}

func writeGrpcError(c *gin.Context, err error) {
	grpcStatus, ok := status.FromError(err)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	httpStatus := http.StatusInternalServerError

	switch grpcStatus.Code() {
	case codes.InvalidArgument:
		httpStatus = http.StatusBadRequest
	case codes.Unauthenticated:
		httpStatus = http.StatusUnauthorized
	case codes.PermissionDenied:
		httpStatus = http.StatusForbidden
	case codes.NotFound:
		httpStatus = http.StatusNotFound
	case codes.Unavailable:
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"message": grpcStatus.Message(),
	})
}
