package httpx

import "mygrep/internal/domain"

type ProcessRequest struct {
	ChunkID   string   `json:"chunk_id"`
	Source    string   `json:"source"`
	StartLine int      `json:"start_line"`
	Lines     []string `json:"lines"`
	Pattern   string   `json:"pattern"`
}

type ProcessResponse struct {
	ChunkID string     `json:"chunk_id"`
	Matches []MatchDTO `json:"matches"`
	Error   string     `json:"error,omitempty"`
}

type MatchDTO struct {
	Source     string `json:"source"`
	LineNumber int    `json:"line_number"`
	Line       string `json:"line"`
}

func toDTO(m domain.Match) MatchDTO {
	return MatchDTO{Source: m.Source, LineNumber: m.LineNumber, Line: m.Line}
}

func fromDTO(d MatchDTO) domain.Match {
	return domain.Match{Source: d.Source, LineNumber: d.LineNumber, Line: d.Line}
}
