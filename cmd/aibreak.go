package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/manuelgilm/idea-breaker/engine"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aibreak",
	Short: "Break down an idea using AI personas",
	Long:  `A CLI tool that breaks down an idea using AI personas (pessimist, optimist, architect, etc.) and synthesizes a scored review.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		idea, _ := cmd.Flags().GetString("idea")
		apiKey, _ := cmd.Flags().GetString("api-key")
		output, _ := cmd.Flags().GetString("output")

		if idea == "" {
			return fmt.Errorf("--idea is required")
		}
		if apiKey == "" {
			apiKey = "sk-placeholder"
		}
		if output == "" {
			return fmt.Errorf("--output is required")
		}

		results := engine.BreakAll(context.Background(), engine.StubCaller{}, idea, apiKey, engine.BuiltInPersonas())

		personas := make([]map[string]any, len(results))
		for i, r := range results {
			p := map[string]any{
				"name":     r.Persona.Name,
				"response": r.Response,
			}
			if r.Err != nil {
				p["error"] = r.Err.Error()
			}
			personas[i] = p
		}

		result := map[string]any{
			"idea":     idea,
			"api_key":  apiKey,
			"personas": personas,
			"status":   "placeholder",
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}

		if err := os.WriteFile(output, data, 0644); err != nil {
			return err
		}

		fmt.Printf("Output written to %s\n", output)
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().String("idea", "", "The idea to break down")
	rootCmd.Flags().String("api-key", "", "API key for the LLM provider (placeholder used if omitted)")
	rootCmd.Flags().String("output", "", "Path to the output JSON file")
}
