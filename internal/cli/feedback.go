package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"aibreak/internal/service"
)

func newFeedbackCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Manage human feedback",
	}

	var author, rationale, aspect string
	var score int
	addCmd := &cobra.Command{
		Use:   "add <idea-id>",
		Short: "Add human feedback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := svc.AddFeedback(cmd.Context(), args[0], author, score, rationale, aspect)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), f.ID)
			return nil
		},
	}
	addCmd.Flags().StringVar(&author, "author", "", "reviewer name/email")
	addCmd.Flags().IntVar(&score, "score", 0, "score 0-5")
	addCmd.Flags().StringVar(&rationale, "rationale", "", "written feedback")
	addCmd.Flags().StringVar(&aspect, "aspect", "", "optional aspect tag")

	var listJSON bool
	listCmd := &cobra.Command{
		Use:   "list <idea-id>",
		Short: "List an idea's feedback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feedbacks, err := svc.ListFeedback(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if listJSON {
				return printJSON(cmd.OutOrStdout(), feedbacks)
			}
			for _, f := range feedbacks {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d/5 — %s\n", f.Author, f.Score, f.Rationale)
			}
			return nil
		},
	}
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output JSON")

	rmCmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete feedback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.DeleteFeedback(cmd.Context(), args[0])
		},
	}

	cmd.AddCommand(addCmd, listCmd, rmCmd)
	return cmd
}
