package engine

import (
	"context"
	"sync"
)

// BreakAll fans out one API call per persona in parallel and returns the
// results in the same order as the input personas. Errored calls are captured
// in Result.Err rather than dropped (collect-all policy).
func BreakAll(ctx context.Context, caller Caller, idea string, personas []Persona) []Result {
	results := make([]Result, len(personas))

	var wg sync.WaitGroup
	for i, p := range personas {
		wg.Add(1)
		go func(i int, p Persona) {
			defer wg.Done()
			resp, err := caller.Chat(ctx, p.SystemPrompt, idea)
			res := Result{Persona: p, Err: err}
			if err == nil {
				bd, parseErr := parseBreakdown(resp)
				if parseErr != nil {
					res.Err = parseErr
				} else {
					res.Breakdown = bd
				}
			}
			results[i] = res
		}(i, p)
	}
	wg.Wait()

	return results
}
