package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"aibreak/internal/domain"
	"aibreak/internal/service"
)

func newRegistryCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage ideas",
	}

	var title, body string
	var tags []string
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Create an idea",
		RunE: func(cmd *cobra.Command, args []string) error {
			idea, warning, err := svc.CreateIdea(cmd.Context(), title, body, tags)
			if err != nil {
				return err
			}
			if warning != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", warning)
			}
			fmt.Fprintln(cmd.OutOrStdout(), idea.ID)
			return nil
		},
	}
	addCmd.Flags().StringVar(&title, "title", "", "idea title")
	addCmd.Flags().StringVar(&body, "body", "", "idea body")
	addCmd.Flags().StringSliceVar(&tags, "tag", nil, "tag (repeatable)")

	var filterTag string
	var listJSON bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List ideas",
		RunE: func(cmd *cobra.Command, args []string) error {
			ideas, err := svc.ListIdeas(cmd.Context(), filterTag)
			if err != nil {
				return err
			}
			if listJSON {
				return printJSON(cmd.OutOrStdout(), ideas)
			}
			for _, i := range ideas {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", i.ID, i.Title)
			}
			return nil
		},
	}
	listCmd.Flags().StringVar(&filterTag, "tag", "", "filter by tag")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output JSON")

	var getJSON bool
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show one idea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idea, err := svc.GetIdea(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if getJSON {
				return printJSON(cmd.OutOrStdout(), idea)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", idea.Title, idea.Body)
			return nil
		},
	}
	getCmd.Flags().BoolVar(&getJSON, "json", false, "output JSON")

	var eTitle, eBody string
	var eTags []string
	editCmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Update an idea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch := domain.IdeaPatch{}
			if cmd.Flags().Changed("title") {
				patch.Title = &eTitle
			}
			if cmd.Flags().Changed("body") {
				patch.Body = &eBody
			}
			if cmd.Flags().Changed("tag") {
				patch.Tags = &eTags
			}
			if _, err := svc.UpdateIdea(cmd.Context(), args[0], patch); err != nil {
				return err
			}
			return nil
		},
	}
	editCmd.Flags().StringVar(&eTitle, "title", "", "new title")
	editCmd.Flags().StringVar(&eBody, "body", "", "new body")
	editCmd.Flags().StringSliceVar(&eTags, "tag", nil, "new tags (repeatable)")

	rmCmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete an idea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.DeleteIdea(cmd.Context(), args[0])
		},
	}

	cmd.AddCommand(addCmd, listCmd, getCmd, editCmd, rmCmd)
	return cmd
}
