// Package actionitem 提供行動項相關 HTTP handlers。
package actionitem

import (
	"time"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
)

type actionItemResponse struct {
	ID          string    `json:"id"`
	MeetingID   string    `json:"meetingId"`
	Description string    `json:"description"`
	Assignee    string    `json:"assignee"`
	DueText     string    `json:"dueText"`
	Done        bool      `json:"done"`
	CreatedAt   time.Time `json:"createdAt"`
}

type pendingActionItemResponse struct {
	actionItemResponse
	MeetingTitle string `json:"meetingTitle"`
}

func toActionItemResponse(a domainactionitem.ActionItem) actionItemResponse {
	return actionItemResponse{
		ID:          a.ID.String(),
		MeetingID:   a.MeetingID.String(),
		Description: a.Description,
		Assignee:    a.Assignee,
		DueText:     a.DueText,
		Done:        a.Done,
		CreatedAt:   a.CreatedAt,
	}
}

func toActionItemResponses(list []domainactionitem.ActionItem) []actionItemResponse {
	out := make([]actionItemResponse, len(list))
	for i, a := range list {
		out[i] = toActionItemResponse(a)
	}
	return out
}

func toPendingActionItemResponses(list []domainactionitem.PendingItem) []pendingActionItemResponse {
	out := make([]pendingActionItemResponse, len(list))
	for i, p := range list {
		out[i] = pendingActionItemResponse{
			actionItemResponse: toActionItemResponse(p.ActionItem),
			MeetingTitle:       p.MeetingTitle,
		}
	}
	return out
}
