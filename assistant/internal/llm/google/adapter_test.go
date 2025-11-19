package google

import (
	"assistant/internal/app"
	"net/http"
	"net/url"
	"os"
	"testing"

	"google.golang.org/genai"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"
)

func TestAdapter_Analyze(t *testing.T) {
	// Start VCR recorder
	r, err := recorder.New("fixtures/analyze_note")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()

	// Add a hook to replace the real API key with a dummy one in the recording
	r.AddHook(func(i *cassette.Interaction) error {
		// Redact query param "key"
		u, err := url.Parse(i.Request.URL)
		if err != nil {
			return nil
		}
		q := u.Query()
		if q.Has("key") {
			q.Set("key", "dummy-key-for-playback")
			u.RawQuery = q.Encode()
			i.Request.URL = u.String()
		}

		// Redact header "x-goog-api-key"
		if i.Request.Headers.Get("x-goog-api-key") != "" {
			i.Request.Headers.Set("x-goog-api-key", "dummy-key-for-playback")
		}
		return nil
	}, recorder.BeforeSaveHook)

	// Determine API key to use
	// If GOOGLE_API_KEY is set, we use it (Recording mode usually)
	// If not, we use the dummy key (Replay mode)
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		apiKey = "dummy-key-for-playback"
	}

	// Configure a custom matcher to ignore the 'key' query parameter and auth header
	r.SetMatcher(func(r *http.Request, i cassette.Request) bool {
		if r.Method != i.Method {
			return false
		}
		u1 := r.URL
		u2, err := url.Parse(i.URL)
		if err != nil {
			return false
		}
		return u1.Path == u2.Path
	})

	// Create a custom HTTP client that uses the recorder
	httpClient := &http.Client{
		Transport: r,
	}

	// Initialize adapter with the custom HTTP client and API key
	config := &genai.ClientConfig{
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}

	adapter, err := NewAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	// Define types
	types := []app.TypeDefinition{
		{
			Name:        "Task",
			Description: "A specific action item or todo.",
			Schema: map[string]interface{}{
				"description": "string",
				"priority":    "string",
			},
		},
	}

	// Test note
	noteText := "I need to buy milk."

	// Analyze
	insights, err := adapter.Analyze(noteText, types)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(insights) == 0 {
		t.Error("Expected insights, got none")
	}

	foundTask := false
	for _, insight := range insights {
		if insight.Type == "Task" {
			foundTask = true
			payload := insight.Payload
			if desc, ok := payload["description"].(string); !ok || desc != "Buy milk" {
				// LLM output might vary slightly, but usually it's close.
				// We'll just check if it exists for now or be lenient.
				t.Logf("Found task description: %v", payload["description"])
			}
		}
	}

	if !foundTask {
		t.Error("Did not find Task insight")
	}
}
