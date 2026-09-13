package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"aibreak/internal/domain"
	"aibreak/internal/service"
)

func newPersonaCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "persona",
		Short: "Manage evaluation personas",
	}

	var listJSON bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available personas",
		RunE: func(cmd *cobra.Command, args []string) error {
			personas, err := svc.ListPersonas(cmd.Context())
			if err != nil {
				return err
			}
			if listJSON {
				return printJSON(cmd.OutOrStdout(), personas)
			}
			for _, p := range personas {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", p.ID, p.Name)
			}
			return nil
		},
	}
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output JSON")

	var name, prompt string
	var weight float64
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Create a custom persona",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := svc.CreatePersona(cmd.Context(), name, prompt, weight)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p.ID)
			return nil
		},
	}
	addCmd.Flags().StringVar(&name, "name", "", "display name")
	addCmd.Flags().StringVar(&prompt, "prompt", "", "system prompt")
	addCmd.Flags().Float64Var(&weight, "weight", 1.0, "score weight")

	var eName, ePrompt string
	var eWeight float64
	editCmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Update a custom persona",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch := domain.PersonaPatch{}
			if cmd.Flags().Changed("name") {
				patch.Name = &eName
			}
			if cmd.Flags().Changed("prompt") {
				patch.SystemPrompt = &ePrompt
			}
			if cmd.Flags().Changed("weight") {
				patch.Weight = &eWeight
			}
			if _, err := svc.UpdatePersona(cmd.Context(), args[0], patch); err != nil {
				return err
			}
			return nil
		},
	}
	editCmd.Flags().StringVar(&eName, "name", "", "new display name")
	editCmd.Flags().StringVar(&ePrompt, "prompt", "", "new system prompt")
	editCmd.Flags().Float64Var(&eWeight, "weight", 1.0, "new score weight")

	rmCmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a custom persona",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.DeletePersona(cmd.Context(), args[0])
		},
	}

	cmd.AddCommand(listCmd, addCmd, editCmd, rmCmd)
	return cmd
}
