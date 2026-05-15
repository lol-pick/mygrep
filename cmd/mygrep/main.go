package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mygrep/internal/adapter/matcher"
	"mygrep/internal/adapter/sink"
	"mygrep/internal/adapter/splitter"
	httpx "mygrep/internal/adapter/transport/http"
	"mygrep/internal/config"
	"mygrep/internal/domain"
	"mygrep/internal/usecase"
)

func main() {
	cfg, err := config.FromArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch cfg.Mode {
	case config.ModeServer:
		runServer(ctx, cfg)
	case config.ModeCoordinator:
		if err := runCoordinator(ctx, cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case config.ModeLocal:
		if err := runLocal(ctx, cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func runServer(ctx context.Context, cfg *config.Config) {
	w := usecase.NewWorker(matcher.NewRegex, cfg.Parallelism)
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           httpx.NewServer(cfg.NodeID, w, cfg.RPCTimeout).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("[%s] worker слушает %s", cfg.NodeID, cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	_ = srv.Shutdown(shCtx)
}

func runCoordinator(ctx context.Context, cfg *config.Config) error {
	remotes := make([]domain.RemoteWorker, 0, len(cfg.Workers))
	for i, addr := range cfg.Workers {
		remotes = append(remotes, httpx.NewClient(
			fmt.Sprintf("w%d", i+1),
			strings.TrimRight(addr, "/"),
			cfg.RPCTimeout,
		))
	}

	sp := splitter.NewLineSplitter(cfg.ChunkLines)
	out := sink.NewStdout(os.Stdout)

	coord, err := usecase.NewCoordinator(usecase.CoordinatorConfig{
		Replication: cfg.Replication,
		Quorum:      cfg.Quorum,
		Parallelism: cfg.Parallelism,
	}, sp, remotes, out)
	if err != nil {
		return err
	}
	return runOnInputs(cfg, func(name string, r io.Reader) error {
		return coord.Run(ctx, name, r, cfg.Pattern)
	})
}

func runLocal(ctx context.Context, cfg *config.Config) error {
	w := usecase.NewWorker(matcher.NewRegex, cfg.Parallelism)
	local := &inProcWorker{id: cfg.NodeID, w: w}

	sp := splitter.NewLineSplitter(cfg.ChunkLines)
	out := sink.NewStdout(os.Stdout)

	coord, err := usecase.NewCoordinator(usecase.CoordinatorConfig{
		Replication: 1, // один локальный — кворум вырождается
		Quorum:      1,
		Parallelism: cfg.Parallelism,
	}, sp, []domain.RemoteWorker{local}, out)
	if err != nil {
		return err
	}
	return runOnInputs(cfg, func(name string, r io.Reader) error {
		return coord.Run(ctx, name, r, cfg.Pattern)
	})
}

type inProcWorker struct {
	id string
	w  *usecase.Worker
}

func (l *inProcWorker) ID() string { return l.id }
func (l *inProcWorker) Submit(ctx context.Context, ch domain.Chunk, pattern string) ([]domain.Match, error) {
	return l.w.Process(ctx, ch, pattern)
}

func runOnInputs(cfg *config.Config, run func(name string, r io.Reader) error) error {
	if len(cfg.Files) == 0 {
		return run("stdin", os.Stdin)
	}
	for _, f := range cfg.Files {
		if err := func() error {
			file, err := os.Open(f)
			if err != nil {
				return err
			}
			defer file.Close()
			return run(f, file)
		}(); err != nil {
			return err
		}
	}
	return nil
}
