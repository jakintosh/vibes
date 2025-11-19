package google

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"assistant/internal/app"

	"google.golang.org/genai"
)

// Adapter implements app.LLMAdapter using Google's Generative AI
type Adapter struct {
	apiKey string
	model  string
	config *genai.ClientConfig
}

// NewAdapter creates a new Google AI adapter
// It expects the API key to be present in the GOOGLE_API_KEY environment variable
// Additional options can be passed via config
func NewAdapter(config *genai.ClientConfig) (*Adapter, error) {
	if config == nil {
		config = &genai.ClientConfig{}
	}

	// If API Key is not set in config, try env var
	if config.APIKey == "" {
		config.APIKey = os.Getenv("GOOGLE_API_KEY")
	}

	// We allow empty API key if custom HTTP client is provided (e.g. for VCR replay),
	// otherwise we expect it.
	if config.APIKey == "" && config.HTTPClient == nil {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable not set")
	}

	return &Adapter{
		apiKey: config.APIKey,
		model:  "gemini-flash-latest",
		config: config,
	}, nil
}

// Analyze processes the note text and extracts insights based on the provided types
func (a *Adapter) Analyze(noteText string, types []app.TypeDefinition) ([]app.Insight, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, a.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	// Note: NewClient does not return a Closer in this SDK version, or we rely on HTTP client management.

	// Construct the prompt
	prompt := a.buildPrompt(noteText, types)

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	resp, err := client.Models.GenerateContent(ctx, a.model, genai.Text(prompt), config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	// Extract JSON string
	var jsonStr string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			jsonStr += part.Text
		}
	}

	// Parse JSON
	var insights []app.Insight
	if err := json.Unmarshal([]byte(jsonStr), &insights); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w\nResponse: %s", err, jsonStr)
	}

	return insights, nil
}

func (a *Adapter) buildPrompt(noteText string, types []app.TypeDefinition) string {
	var typeDefs []string
	for _, t := range types {
		schemaBytes, _ := json.MarshalIndent(t.Schema, "", "  ")
		typeDefs = append(typeDefs, fmt.Sprintf("Type: %s\nDescription: %s\nSchema:\n%s", t.Name, t.Description, string(schemaBytes)))
	}

	return fmt.Sprintf(`Analyze the following note and extract insights matching the defined types.
Return a JSON array of objects, where each object has a "type" field (matching one of the defined Type names) and a "payload" field containing the extracted data matching the schema.

Defined Types:
%s

Note Content:
%s

Output JSON:`, strings.Join(typeDefs, "\n\n"), noteText)
}
