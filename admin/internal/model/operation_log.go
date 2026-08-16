package model

type OperationLogModel struct {
	BaseModel
	AdminID    uint   `gorm:"index;not null;comment:操作管理员ID" json:"admin_id"`
	AdminName  string `gorm:"size:64;not null;comment:操作人账号" json:"admin_name"`
	Module     string `gorm:"size:64;comment:操作模块" json:"module"`
	OperType   string `gorm:"size:32;comment:操作类型" json:"oper_type"`
	OperDesc   string `gorm:"size:512;comment:操作描述" json:"oper_desc"`
	RequestURL string `gorm:"size:255;comment:请求接口地址" json:"request_url"`
	Method     string `gorm:"size:16;comment:请求方式 GET/POST/PUT/DELETE" json:"method"`
	IP         string `gorm:"size:64;comment:操作IP地址" json:"ip"`
	UserAgent  string `gorm:"size:512;comment:浏览器客户端" json:"user_agent"`
	Params     string `gorm:"type:text;comment:请求参数JSON" json:"params"`
	Result     string `gorm:"type:text;comment:返回结果JSON" json:"result"`
	Status     int8   `gorm:"default:1;comment:操作状态 1成功 0失败" json:"status"`

	// 关联管理员
	Admin AdminModel `gorm:"foreignKey:AdminID" json:"admin,omitempty"`
}

func (OperationLogModel) TableName() string {
	return "operation_log"
}
