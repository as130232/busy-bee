package qa

import (
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

type sourceResponse struct {
	Index     int    `json:"index"`
	MeetingID string `json:"meetingId"`
	Title     string `json:"title"`
	Snippet   string `json:"snippet"`
}

type answerResponse struct {
	Answer  string           `json:"answer"`
	NoMatch bool             `json:"noMatch"`
	Sources []sourceResponse `json:"sources"`
}

func toAnswerResponse(r domainsearch.QAResult) answerResponse {
	sources := make([]sourceResponse, 0, len(r.Sources))
	for _, s := range r.Sources {
		sources = append(sources, sourceResponse{
			Index:     s.Index,
			MeetingID: s.MeetingID.String(),
			Title:     s.Title,
			Snippet:   s.Content,
		})
	}
	return answerResponse{Answer: r.Answer, NoMatch: r.NoMatch, Sources: sources}
}
