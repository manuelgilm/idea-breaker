package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/manuelgilm/idea-breaker/engine"
	"github.com/manuelgilm/idea-breaker/persona"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aibreak",
	Short: "Break down an idea using AI personas",
	Long:  `A CLI tool that breaks down an idea using AI personas (pessimist, optimist, architect, etc.) and synthesizes a scored review.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		idea, _ := cmd.Flags().GetString("idea")
		output, _ := cmd.Flags().GetString("output")
		mock, _ := cmd.Flags().GetBool("mock")

		if idea == "" {
			return fmt.Errorf("--idea is required")
		}
		if output == "" {
			return fmt.Errorf("--output is required")
		}

		var caller engine.Caller = engine.MockCaller{}
		if !mock {
			apiKey := os.Getenv("OPENAI_API_KEY")
			if apiKey == "" {
				return fmt.Errorf("OPENAI_API_KEY is not set")
			}
			caller = engine.NewHTTPCaller(apiKey, "", "")
		}

		var persons []engine.Persona
		var err error
		var synthPrompt string
		if dir := os.Getenv("AIBREAK_PERSONAS_DIR"); dir != "" {
			persons, err = persona.FileSource{Dir: dir}.Load()
			if err != nil {
				return err
			}
			synthPrompt, err = persona.LoadSynthesizer(dir)
			if err != nil {
				return err
			}
		} else {
			persons, err = persona.EmbeddedSource{}.Load()
			if err != nil {
				return err
			}
			synthPrompt, err = persona.LoadEmbeddedSynthesizer()
			if err != nil {
				return err
			}
		}

		results := engine.BreakAll(context.Background(), caller, idea, persons)

		synthesis, err := engine.Synthesize(context.Background(), caller, synthPrompt, idea, results)
		if err != nil {
			return err
		}

		personas := make([]map[string]any, len(results))
		for i, r := range results {
			p := map[string]any{
				"name":         r.Persona.Name,
				"summary":      r.Breakdown.Summary,
				"key_points":   r.Breakdown.KeyPoints,
				"risks":        r.Breakdown.Risks,
				"dependencies": r.Breakdown.Dependencies,
				"score":        r.Breakdown.Score,
			}
			if r.Err != nil {
				p["error"] = r.Err.Error()
			}
			personas[i] = p
		}

		result := map[string]any{
			"idea":     idea,
			"personas": personas,
			"synthesis": map[string]any{
				"feedback": synthesis.Feedback,
				"score":    synthesis.Score,
			},
			"status": "complete",
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
	rootCmd.Flags().String("output", "", "Path to the output JSON file")
	rootCmd.Flags().Bool("mock", false, "Run offline with mock persona and synthesizer responses (no API key needed)")
}
