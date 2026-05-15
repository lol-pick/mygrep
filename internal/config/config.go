package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type Mode string

const (
	ModeLocal       Mode = "local"
	ModeServer      Mode = "server"
	ModeCoordinator Mode = "coordinator"
)

type Config struct {
	Mode        Mode
	Pattern     string
	Files       []string
	Workers     []string
	Listen      string
	NodeID      string
	Replication int
	Quorum      int
	ChunkLines  int
	Parallelism int
	RPCTimeout  time.Duration
}

func FromArgs(args []string) (*Config, error) {
	fs := flag.NewFlagSet("mygrep", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Использование:")
		fmt.Fprintln(fs.Output(), "  mygrep [флаги] [файл ...]")
		fs.PrintDefaults()
	}

	var (
		mode    = fs.String("mode", "local", "режим: local | server | coordinator")
		pattern = fs.String("e", "", "regex-паттерн для поиска (RE2)")
		workers = fs.String("workers", "", "адреса воркеров через запятую (для coordinator)")
		listen  = fs.String("listen", ":8081", "HTTP-адрес для server")
		nodeID  = fs.String("id", "", "идентификатор узла (по умолчанию node-<pid>)")
		repl    = fs.Int("replication", 1, "сколько воркеров обрабатывают один чанк")
		quorum  = fs.Int("quorum", 0, "минимум согласных ответов; 0 => replication/2+1")
		chunkLn = fs.Int("chunk", 1000, "размер чанка в строках")
		par     = fs.Int("parallelism", 4, "параллельных чанков / горутин на чанк")
		rpcSec  = fs.Int("rpc-timeout", 30, "таймаут запроса к воркеру, сек")
	)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := &Config{
		Mode:        Mode(*mode),
		Pattern:     *pattern,
		Files:       fs.Args(),
		Listen:      *listen,
		NodeID:      *nodeID,
		Replication: *repl,
		Quorum:      *quorum,
		ChunkLines:  *chunkLn,
		Parallelism: *par,
		RPCTimeout:  time.Duration(*rpcSec) * time.Second,
	}
	if *workers != "" {
		for _, w := range strings.Split(*workers, ",") {
			if w = strings.TrimSpace(w); w != "" {
				cfg.Workers = append(cfg.Workers, w)
			}
		}
	}
	if cfg.NodeID == "" {
		cfg.NodeID = fmt.Sprintf("node-%d", os.Getpid())
	}
	if cfg.Quorum == 0 {
		cfg.Quorum = cfg.Replication/2 + 1
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch c.Mode {
	case ModeLocal:
		if c.Pattern == "" {
			return errors.New("требуется -e <pattern>")
		}
	case ModeServer:
	case ModeCoordinator:
		if c.Pattern == "" {
			return errors.New("требуется -e <pattern>")
		}
		if len(c.Workers) == 0 {
			return errors.New("требуется -workers")
		}
		if c.Replication > len(c.Workers) {
			return fmt.Errorf("replication=%d, но воркеров %d", c.Replication, len(c.Workers))
		}
		if c.Quorum > c.Replication {
			return fmt.Errorf("quorum=%d > replication=%d", c.Quorum, c.Replication)
		}
	default:
		return fmt.Errorf("неизвестный режим: %s", c.Mode)
	}
	return nil
}
