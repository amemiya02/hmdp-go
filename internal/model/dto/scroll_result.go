package dto

// ScrollResult 滚动分页返回结果
type ScrollResult struct {
	List    interface{} `json:"list"`    // 数据列表
	MinTime int64       `json:"minTime"` // 本次查询的最小时间戳（作为下一次查询的 max）
	Offset  int         `json:"offset"`  // 偏移量（用于跳过 score 相同的重复元素）
}
