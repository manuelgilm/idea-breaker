package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"aibreak/internal/service"
)

func newResourceCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Manage research resources",
	}

	var url, title, kind, note string
	addCmd := &cobra.Command{
		Use:   "add <idea-id>",
		Short: "Attach a research resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := svc.AddResource(cmd.Context(), args[0], url, title, kind, note)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), r.ID)
			return nil
		},
	}
	addCmd.Flags().StringVar(&url, "url", "", "resource URL")
	addCmd.Flags().StringVar(&title, "title", "", "optional title")
	addCmd.Flags().StringVar(&kind, "kind", "", "kind (article|repo|paper|video|other)")
	addCmd.Flags().StringVar(&note, "note", "", "why it is relevant")

	var listJSON bool
	listCmd := &cobra.Command{
		Use:   "list <idea-id>",
		Short: "List an idea's resources",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resources, err := svc.ListResources(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if listJSON {
				return printJSON(cmd.OutOrStdout(), resources)
			}
			for _, r := range resources {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", r.Kind, r.URL)
			}
			return nil
		},
	}
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output JSON")

	rmCmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.DeleteResource(cmd.Context(), args[0])
		},
	}

	cmd.AddCommand(addCmd, listCmd, rmCmd)
	return cmd
}
