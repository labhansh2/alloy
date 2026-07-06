// Package ai is a minimal client for the OpenRouter API.
//
// Example:
//
//	c := ai.New(os.Getenv("OPENROUTER_API_KEY"), clients.WithHTTPClient(httpClient))
//	resp, err := c.ChatCompletion(ctx, ai.ChatCompletionParams{
//		Model: "openai/gpt-4o",
//		Messages: []ai.Message{{Role: "user", Content: "Hello"}},
//	})
//	fmt.Println(resp.Content())
package ai
