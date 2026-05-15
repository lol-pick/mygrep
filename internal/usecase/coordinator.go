package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"mygrep/internal/domain"
)

type CoordinatorConfig struct {
	Replication int
	Quorum      int
	Parallelism int
}

type Coordinator struct {
	cfg      CoordinatorConfig
	splitter domain.Splitter
	workers  []domain.RemoteWorker
	sink     domain.ResultSink
}

func NewCoordinator(
	cfg CoordinatorConfig,
	s domain.Splitter,
	ws []domain.RemoteWorker,
	sink domain.ResultSink,
) (*Coordinator, error) {
	if cfg.Replication < 1 {
		return nil, errors.New("replication must be >= 1")
	}
	if cfg.Quorum < 1 || cfg.Quorum > cfg.Replication {
		return nil, fmt.Errorf("quorum must be in [1, %d]", cfg.Replication)
	}
	if len(ws) < cfg.Replication {
		return nil, fmt.Errorf("need at least %d workers, got %d", cfg.Replication, len(ws))
	}
	if cfg.Parallelism < 1 {
		cfg.Parallelism = 1
	}
	return &Coordinator{cfg: cfg, splitter: s, workers: ws, sink: sink}, nil
}

func (c *Coordinator) Run(ctx context.Context, source string, r io.Reader, pattern string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chunks, splitErrs := c.splitter.Split(ctx, source, r)
	quorum := &Quorum{Required: c.cfg.Quorum}

	var (
		mu         sync.Mutex
		results    = map[string][]domain.Match{}
		chunkOrder []string
		firstErr   error
		errOnce    sync.Once
	)
	setErr := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	sem := make(chan struct{}, c.cfg.Parallelism)
	var wg sync.WaitGroup

	var (
		rrIdx int
		rrMu  sync.Mutex
	)
	pickReplicas := func() []domain.RemoteWorker {
		rrMu.Lock()
		defer rrMu.Unlock()
		picked := make([]domain.RemoteWorker, c.cfg.Replication)
		for i := 0; i < c.cfg.Replication; i++ {
			picked[i] = c.workers[(rrIdx+i)%len(c.workers)]
		}
		rrIdx = (rrIdx + 1) % len(c.workers)
		return picked
	}

	for chunks != nil || splitErrs != nil {
		select {
		case <-ctx.Done():
			chunks, splitErrs = nil, nil
		case err, ok := <-splitErrs:
			if !ok {
				splitErrs = nil
				continue
			}
			if err != nil {
				setErr(fmt.Errorf("split: %w", err))
			}
		case ch, ok := <-chunks:
			if !ok {
				chunks = nil
				continue
			}
			mu.Lock()
			chunkOrder = append(chunkOrder, ch.ID)
			mu.Unlock()

			sem <- struct{}{}
			wg.Add(1)
			go func(ch domain.Chunk) {
				defer wg.Done()
				defer func() { <-sem }()

				replicas := pickReplicas()
				merged, err := c.processChunk(ctx, ch, pattern, replicas, quorum)
				if err != nil {
					setErr(fmt.Errorf("chunk %s: %w", ch.ID, err))
					return
				}
				mu.Lock()
				results[ch.ID] = merged
				mu.Unlock()
			}(ch)
		}
	}

	wg.Wait()
	if firstErr != nil {
		return firstErr
	}

	for _, id := range chunkOrder {
		for _, m := range results[id] {
			if err := c.sink.Write(m); err != nil {
				return err
			}
		}
	}
	return c.sink.Flush()
}

func (c *Coordinator) processChunk(
	ctx context.Context,
	chunk domain.Chunk,
	pattern string,
	replicas []domain.RemoteWorker,
	q *Quorum,
) ([]domain.Match, error) {
	type resp struct {
		matches []domain.Match
		err     error
	}
	out := make(chan resp, len(replicas))
	for _, w := range replicas {
		go func(w domain.RemoteWorker) {
			ms, err := w.Submit(ctx, chunk, pattern)
			out <- resp{matches: ms, err: err}
		}(w)
	}

	var (
		responses [][]domain.Match
		lastErr   error
		failed    int
	)
	for i := 0; i < len(replicas); i++ {
		r := <-out
		if r.err != nil {
			failed++
			lastErr = r.err
			continue
		}
		responses = append(responses, r.matches)
	}

	if len(responses) < q.Required {
		return nil, fmt.Errorf(
			"кворум не достигнут: успешных %d из %d (требуется %d), последняя ошибка: %v",
			len(responses), len(replicas), q.Required, lastErr,
		)
	}
	return q.Reconcile(responses), nil
}
