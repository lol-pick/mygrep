package domain

import "fmt"

type Match struct {
	Source     string
	LineNumber int
	Line       string
}

func (m Match) Key() string {
	return fmt.Sprintf("%s\x00%d\x00%s", m.Source, m.LineNumber, m.Line)
}
