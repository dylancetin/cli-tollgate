package core

import (
	"fmt"
	"runtime"
)

// Snippet is a copy-ready setup block for a client.
type Snippet struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Snippets returns setup blocks for common clients. On Windows they use
// PowerShell syntax and curl.exe (plain `curl` is Invoke-WebRequest there).
func Snippets(baseURL, apiKey string) []Snippet {
	if runtime.GOOS == "windows" {
		return powershellSnippets(baseURL, apiKey)
	}
	return shellSnippets(baseURL, apiKey)
}

func powershellSnippets(baseURL, apiKey string) []Snippet {
	return []Snippet{
		{"claudeCode", "Claude Code (PowerShell)", fmt.Sprintf(
			"$env:ANTHROPIC_BASE_URL = \"%s\"\n$env:ANTHROPIC_AUTH_TOKEN = \"%s\"\nclaude", baseURL, apiKey)},
		{"claudeCodePersistent", "Claude Code (all new terminals)", fmt.Sprintf(
			"setx ANTHROPIC_BASE_URL \"%s\"\nsetx ANTHROPIC_AUTH_TOKEN \"%s\"\n# open a new terminal, then run: claude", baseURL, apiKey)},
		{"codex", "Codex CLI", fmt.Sprintf(`# %%USERPROFILE%%\.codex\config.toml
model_provider = "tollgate"

[model_providers.tollgate]
name = "CLI Tollgate"
base_url = "%s/v1"
env_key = "TOLLGATE_API_KEY"
wire_api = "responses"

# then in PowerShell:
# setx TOLLGATE_API_KEY "%s"`, baseURL, apiKey)},
		{"openAI", "OpenAI-compatible SDK (PowerShell)", fmt.Sprintf(
			"$env:OPENAI_BASE_URL = \"%s/v1\"\n$env:OPENAI_API_KEY = \"%s\"", baseURL, apiKey)},
		{"webFetch", "Web page (fetch)", fetchSnippet(baseURL, apiKey)},
		{"curl", "curl test", fmt.Sprintf(`curl.exe -s %s/v1/models -H "Authorization: Bearer %s"`, baseURL, apiKey)},
	}
}

func shellSnippets(baseURL, apiKey string) []Snippet {
	return []Snippet{
		{"claudeCode", "Claude Code", fmt.Sprintf(
			"export ANTHROPIC_BASE_URL=\"%s\"\nexport ANTHROPIC_AUTH_TOKEN=\"%s\"", baseURL, apiKey)},
		{"codex", "Codex CLI", fmt.Sprintf(`# ~/.codex/config.toml
model_provider = "tollgate"

[model_providers.tollgate]
name = "CLI Tollgate"
base_url = "%s/v1"
env_key = "TOLLGATE_API_KEY"
wire_api = "responses"

# then in your shell:
# export TOLLGATE_API_KEY="%s"`, baseURL, apiKey)},
		{"openAI", "OpenAI-compatible SDK", fmt.Sprintf(
			"export OPENAI_BASE_URL=\"%s/v1\"\nexport OPENAI_API_KEY=\"%s\"", baseURL, apiKey)},
		{"webFetch", "Web page (fetch)", fetchSnippet(baseURL, apiKey)},
		{"curl", "curl test", fmt.Sprintf(`curl -s %s/v1/models -H "Authorization: Bearer %s"`, baseURL, apiKey)},
	}
}

func fetchSnippet(baseURL, apiKey string) string {
	return fmt.Sprintf(`const res = await fetch("%s/v1/chat/completions", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "Authorization": "Bearer %s",
  },
  body: JSON.stringify({
    model: "gpt-5",
    messages: [{ role: "user", content: "Hello" }],
  }),
});
console.log(await res.json());`, baseURL, apiKey)
}
