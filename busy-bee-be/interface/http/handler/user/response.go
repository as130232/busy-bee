package user

import (
	"time"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	// LiveMicTranscription 即時會議是否同時即時辨識麥克風（設定頁開關）。
	LiveMicTranscription bool      `json:"liveMicTranscription"`
	CreatedAt            time.Time `json:"createdAt"`
}

func toUserResponse(u domainuser.User) userResponse {
	return userResponse{
		ID:                   u.ID.String(),
		Email:                u.Email,
		DisplayName:          u.DisplayName,
		AvatarURL:            u.AvatarURL,
		LiveMicTranscription: u.LiveMicTranscription,
		CreatedAt:            u.CreatedAt,
	}
}
