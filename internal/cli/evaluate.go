package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"aibreak/internal/domain"
	"aibreak/internal/engine"
	"aibreak/internal/service"
)

func newEvaluateCmd(svc *service.Service) *cobra.Command {
	var personas string
	var jsonOut bool
	var summarize bool
	cmd := &cobra.Command{
		Use:   "evaluate <id>",
		Short: "Evaluate an idea with personas",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := splitIDs(personas)
			score, err := svc.Evaluate(cmd.Context(), args[0], ids, summarize)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(cmd.OutOrStdout(), score)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Total: %.1f, spread: %.0f/5 (%s) (%d/%d personas)\n",
				score.Total, score.Spread, engine.Agreement(score.Spread), score.Responded, score.Requested)
			for _, ev := range score.Breakdown {
				if ev.Status == domain.StatusSuccess {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d/5 — %s\n", ev.PersonaID, ev.Score, ev.Rationale)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: failed — %s\n", ev.PersonaID, ev.Error)
				}
			}
			if score.Verdict != "" || score.Summary != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Verdict: %s\n%s\n", score.Verdict, score.Summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&personas, "personas", "", "comma-separated persona ids (default: all)")
	cmd.Flags().BoolVar(&summarize, "summary", false, "synthesize a consolidated verdict (extra LLM call)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")
	return cmd
}

func newHistoryCmd(svc *service.Service) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "history <id>",
		Short: "List an idea's past evaluations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runs, err := svc.ListRuns(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(cmd.OutOrStdout(), runs)
			}
			for _, r := range runs {
				fmt.Fprintf(cmd.OutOrStdout(), "run %s: total %.1f, spread: %.0f/5 (%s) (%d/%d)",
					r.RunID, r.Total, r.Spread, engine.Agreement(r.Spread), r.Responded, r.Requested)
				if r.Verdict != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " verdict %s", r.Verdict)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")
	return cmd
}

func splitIDs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
