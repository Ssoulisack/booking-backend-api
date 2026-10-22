package dto

// BaseResponse represents the standard response structure for all API responses
type BaseResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta represents pagination metadata
type Meta struct {
	Total     int `json:"total"`
	Page      int `json:"page"`
	Limit     int `json:"limit"`
	TotalPage int `json:"total_page"`
}
type PaginationQuery struct {
	Page        int    `json:"page,omitempty" query:"page"`
	Limit       int    `json:"limit,omitempty" query:"limit"`
	Keyword     string `json:"keyword,omitempty" query:"keyword"`
	RoleID      uint   `json:"roleID,omitempty" query:"roleID"`
	TenantID    uint   `json:"tenantID,omitempty" query:"tenantID"`
	UserID      uint   `json:"userID,omitempty" query:"userID"`
	Status      string `json:"status,omitempty" query:"status"`
	Sort        string `json:"sort,omitempty" query:"sort"`
	WarehouseID uint   `json:"warehouseID,omitempty" query:"warehouseID"`
	Type        string `json:"type,omitempty" query:"type"`
	LocationID  uint   `json:"locationID,omitempty" query:"locationID"`
	TypeID      uint   `json:"typeID,omitempty" query:"typeID"`
	ZoneID      uint   `json:"zoneID,omitempty" query:"zoneID"`
}
