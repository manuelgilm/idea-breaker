// Package cli implements the aibreak command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"aibreak/internal/service"
)

// New builds the aibreak root command wired to svc.
func New(svc *service.Service) *cobra.Command {
	root := &cobra.Command{
		Use:           "aibreak",
		Short:         "AI-powered idea evaluation",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newEvaluateCmd(svc),
		newHistoryCmd(svc),
		newRegistryCmd(svc),
		newPersonaCmd(svc),
		newFeedbackCmd(svc),
		newResourceCmd(svc),
	)
	return root
}

func printJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
