package usecase

import "mygrep/internal/domain"

type Quorum struct {
	Required int
}

func (q *Quorum) Reconcile(responses [][]domain.Match) []domain.Match {
	type entry struct {
		m     domain.Match
		votes int
	}
	bucket := map[string]*entry{}
	order := []string{}

	for _, set := range responses {
		seen := make(map[string]struct{}, len(set))
		for _, m := range set {
			k := m.Key()
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}

			if e, ok := bucket[k]; ok {
				e.votes++
			} else {
				bucket[k] = &entry{m: m, votes: 1}
				order = append(order, k)
			}
		}
	}

	out := make([]domain.Match, 0, len(order))
	for _, k := range order {
		if bucket[k].votes >= q.Required {
			out = append(out, bucket[k].m)
		}
	}
	return out
}
