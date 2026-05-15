package splitter

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"

	"mygrep/internal/domain"
)

type LineSplitter struct {
	ChunkLines int
}

func NewLineSplitter(chunkLines int) *LineSplitter {
	if chunkLines < 1 {
		chunkLines = 1000
	}
	return &LineSplitter{ChunkLines: chunkLines}
}

func (s *LineSplitter) Split(ctx context.Context, source string, r io.Reader) (<-chan domain.Chunk, <-chan error) {
	chunks := make(chan domain.Chunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)

		var (
			buf       []string
			startLine = 1
			currLine  = 1
		)
		emit := func() {
			if len(buf) == 0 {
				return
			}
			ch := domain.Chunk{
				ID:        randID(),
				Source:    source,
				StartLine: startLine,
				Lines:     buf,
			}
			select {
			case <-ctx.Done():
			case chunks <- ch:
			}
			startLine = currLine
			buf = nil
		}

		for sc.Scan() {
			if ctx.Err() != nil {
				return
			}
			buf = append(buf, sc.Text())
			currLine++
			if len(buf) >= s.ChunkLines {
				emit()
			}
		}
		if err := sc.Err(); err != nil {
			errs <- err
			return
		}
		emit() // хвост короче ChunkLines
	}()

	return chunks, errs
}

func randID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
