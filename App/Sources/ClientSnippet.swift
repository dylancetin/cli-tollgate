import Foundation

/// Copy-ready setup snippets for common clients.
enum ClientSnippet: String, CaseIterable, Identifiable {
    case claudeCode
    case codex
    case openAI
    case webFetch
    case curl

    var id: String { rawValue }

    var title: String {
        switch self {
        case .claudeCode: return "Claude Code"
        case .codex: return "Codex CLI"
        case .openAI: return "OpenAI-compatible SDK"
        case .webFetch: return "Web page (fetch)"
        case .curl: return "curl test"
        }
    }

    func text(baseURL: String, apiKey: String) -> String {
        switch self {
        case .claudeCode:
            return """
            export ANTHROPIC_BASE_URL="\(baseURL)"
            export ANTHROPIC_AUTH_TOKEN="\(apiKey)"
            """
        case .codex:
            return """
            # ~/.codex/config.toml
            model_provider = "tollgate"

            [model_providers.tollgate]
            name = "CLI Tollgate"
            base_url = "\(baseURL)/v1"
            env_key = "TOLLGATE_API_KEY"
            wire_api = "responses"

            # then in your shell:
            # export TOLLGATE_API_KEY="\(apiKey)"
            """
        case .openAI:
            return """
            export OPENAI_BASE_URL="\(baseURL)/v1"
            export OPENAI_API_KEY="\(apiKey)"
            """
        case .webFetch:
            return """
            const res = await fetch("\(baseURL)/v1/chat/completions", {
              method: "POST",
              headers: {
                "Content-Type": "application/json",
                "Authorization": "Bearer \(apiKey)",
              },
              body: JSON.stringify({
                model: "gpt-5",
                messages: [{ role: "user", content: "Hello" }],
              }),
            });
            console.log(await res.json());
            """
        case .curl:
            return """
            curl -s \(baseURL)/v1/models -H "Authorization: Bearer \(apiKey)"
            """
        }
    }
}
