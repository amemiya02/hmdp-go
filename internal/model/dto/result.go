package dto

// Result 统一响应结构体
type Result struct {
	Success  bool        `json:"success"`  // 是否成功
	ErrorMsg string      `json:"errorMsg"` // 错误信息
	Data     interface{} `json:"data"`     // 响应数据
	Total    int64       `json:"total"`    // 总数（分页用）
}

// Ok 成功响应（无数据）
func Ok() *Result {
	return &Result{
		Success:  true,
		ErrorMsg: "",
		Data:     nil,
		Total:    0,
	}
}

// OkWithData 成功响应（带数据）
func OkWithData(data interface{}) *Result {
	return &Result{
		Success:  true,
		ErrorMsg: "",
		Data:     data,
		Total:    0,
	}
}

// OkWithList 成功响应（分页列表）
func OkWithList(data interface{}, total int64) *Result {
	return &Result{
		Success:  true,
		ErrorMsg: "",
		Data:     data,
		Total:    total,
	}
}

// Fail 失败响应
func Fail(errorMsg string) *Result {
	return &Result{
		Success:  false,
		ErrorMsg: errorMsg,
		Data:     nil,
		Total:    0,
	}
}
