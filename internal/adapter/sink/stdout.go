package sink

import (
	"bufio"
	"fmt"
	"io"

	"mygrep/internal/domain"
)

type Stdout struct {
	w *bufio.Writer
}

func NewStdout(w io.Writer) *Stdout {
	return &Stdout{w: bufio.NewWriter(w)}
}

func (s *Stdout) Write(m domain.Match) error {
	_, err := fmt.Fprintf(s.w, "%s:%d:%s\n", m.Source, m.LineNumber, m.Line)
	return err
}

func (s *Stdout) Flush() error { return s.w.Flush() }
