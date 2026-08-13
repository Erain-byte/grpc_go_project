package handler

import (
	"admin/internal/svc"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
)

type AdminHandler struct {
	adminv1.UnimplementedAdminServiceServer
	svcCtx *svc.ServiceContext
}

func NewAdminHandler(svcCtx *svc.ServiceContext) *AdminHandler {
	return &AdminHandler{svcCtx: svcCtx}
}
