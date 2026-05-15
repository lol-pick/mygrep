package domain

import (
	"context"
	"io"
)

type Pattern interface {
	Matches(line string) bool
	String() string
}

type Splitter interface {
	Split(ctx context.Context, source string, r io.Reader) (<-chan Chunk, <-chan error)
}

type RemoteWorker interface {
	ID() string
	Submit(ctx context.Context, chunk Chunk, pattern string) ([]Match, error)
}

type ResultSink interface {
	Write(m Match) error
	Flush() error
}
