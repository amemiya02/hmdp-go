package dto

type UserDTO struct {
	ID       uint64 `json:"id"`
	NickName string `json:"nickName"`
	Icon     string `json:"icon"`
}
