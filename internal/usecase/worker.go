package usecase

import (
	"context"
	"sort"

	"mygrep/internal/domain"
)

type PatternFactory func(string) (domain.Pattern, error)

type Worker struct {
	pf          PatternFactory
	parallelism int
}

func NewWorker(pf PatternFactory, parallelism int) *Worker {
	if parallelism < 1 {
		parallelism = 1
	}
	return &Worker{pf: pf, parallelism: parallelism}
}

func (w *Worker) Process(ctx context.Context, chunk domain.Chunk, patternStr string) ([]domain.Match, error) {
	p, err := w.pf(patternStr)
	if err != nil {
		return nil, err
	}
	if len(chunk.Lines) == 0 {
		return nil, nil
	}

	n := w.parallelism
	if n > len(chunk.Lines) {
		n = len(chunk.Lines)
	}
	blockSize := (len(chunk.Lines) + n - 1) / n

	type result struct {
		start   int
		matches []domain.Match
	}
	out := make(chan result, n)

	for i := 0; i < n; i++ {
		start := i * blockSize
		end := start + blockSize
		if end > len(chunk.Lines) {
			end = len(chunk.Lines)
		}
		go func(start, end int) {
			var local []domain.Match
			for j := start; j < end; j++ {
				if ctx.Err() != nil {
					break
				}
				if p.Matches(chunk.Lines[j]) {
					local = append(local, domain.Match{
						Source:     chunk.Source,
						LineNumber: chunk.StartLine + j,
						Line:       chunk.Lines[j],
					})
				}
			}
			out <- result{start: start, matches: local}
		}(start, end)
	}

	parts := make([]result, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, <-out)
	}
	sort.Slice(parts, func(a, b int) bool { return parts[a].start < parts[b].start })

	var all []domain.Match
	for _, pt := range parts {
		all = append(all, pt.matches...)
	}
	return all, ctx.Err()
}
