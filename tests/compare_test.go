package tests

import (
	"bytes"
	"context"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"mygrep/internal/adapter/matcher"
	"mygrep/internal/adapter/sink"
	"mygrep/internal/adapter/splitter"
	"mygrep/internal/domain"
	"mygrep/internal/usecase"
)

type inProcWorker struct {
	id string
	w  *usecase.Worker
}

func (l *inProcWorker) ID() string { return l.id }
func (l *inProcWorker) Submit(ctx context.Context, ch domain.Chunk, pattern string) ([]domain.Match, error) {
	return l.w.Process(ctx, ch, pattern)
}

func TestCompareWithSystemGrep(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("системный grep недоступен")
	}

	input := strings.Join([]string{
		"apple",
		"banana",
		"applesauce",
		"cherry",
		"ananas",
		"kiwi",
		"pineapple",
		"grape",
		"appletini",
	}, "\n")
	pattern := "apple"

	cmd := exec.Command("grep", "-nE", pattern)
	cmd.Stdin = strings.NewReader(input)
	var sysOut bytes.Buffer
	cmd.Stdout = &sysOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("system grep: %v", err)
	}

	var buf bytes.Buffer
	out := sink.NewStdout(&buf)
	w := usecase.NewWorker(matcher.NewRegex, 2)
	workers := []domain.RemoteWorker{
		&inProcWorker{id: "a", w: w},
		&inProcWorker{id: "b", w: w},
		&inProcWorker{id: "c", w: w},
	}
	coord, err := usecase.NewCoordinator(usecase.CoordinatorConfig{
		Replication: 3, Quorum: 2, Parallelism: 2,
	}, splitter.NewLineSplitter(2), workers, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := coord.Run(context.Background(), "stdin", strings.NewReader(input), pattern); err != nil {
		t.Fatal(err)
	}

	wantSet := normalize(sysOut.String())
	gotSet := normalize(buf.String())
	if !equalSlices(wantSet, gotSet) {
		t.Errorf("несовпадение наборов совпадений\nожидалось:\n%v\nполучено:\n%v",
			wantSet, gotSet)
	}
}

func normalize(s string) []string {
	parts := strings.Split(strings.TrimSpace(s), "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		first := strings.SplitN(p, ":", 2)
		if len(first) == 2 && !isDigits(first[0]) {
			p = first[1]
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestQuorumDropsMinorityOpinion(t *testing.T) {
	q := &usecase.Quorum{Required: 2}
	common := domain.Match{Source: "f", LineNumber: 1, Line: "apple"}
	odd := domain.Match{Source: "f", LineNumber: 2, Line: "BUG"}
	got := q.Reconcile([][]domain.Match{
		{common},
		{common, odd},
		{common},
	})
	if len(got) != 1 || got[0] != common {
		t.Fatalf("ожидалось только %v, получили %v", common, got)
	}
}
