package correction

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// Queue 校正背景佇列。
type Queue interface {
	EnqueueCorrection(ctx context.Context, correctionID uuid.UUID) error
}

// MeetingReader 以 owner 條件讀取紀錄（domain/meeting.Repository 的子集）。
type MeetingReader interface {
	GetForUser(ctx context.Context, id, userID uuid.UUID) (meeting.Meeting, error)
}

// ActionItemLister 列出紀錄的行動項（domain/actionitem.Repository 的子集）。
type ActionItemLister interface {
	ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]actionitem.ActionItem, error)
}

// Indexer 重建紀錄的語意搜尋索引（application/search.IndexUC）。
type Indexer interface {
	Execute(ctx context.Context, meetingID uuid.UUID) error
}

// AudioDownloader 讀取已上傳音檔（domain/meeting.AudioStorage 的子集）。
type AudioDownloader interface {
	Download(ctx context.Context, objectPath string) (io.ReadCloser, int64, error)
}
