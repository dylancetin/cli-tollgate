import Foundation

/// Thin client for the engine's /v8/management API on loopback.
struct ManagementClient {
    let port: Int
    let password: String

    var baseURL: URL { URL(string: "http://127.0.0.1:\(port)")! }

    private static let session: URLSession = {
        let config = URLSessionConfiguration.ephemeral
        config.timeoutIntervalForRequest = 15
        config.connectionProxyDictionary = [:] // never route loopback through a system proxy
        return URLSession(configuration: config)
    }()

    // MARK: - Generic requests

    func get<T: Decodable>(_ path: String, query: [String: String] = [:], as type: T.Type = T.self) async throws -> T {
        let data = try await send("GET", path, query: query)
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw APIError.decoding(path, error)
        }
    }

    func put<Body: Encodable>(_ path: String, _ body: Body) async throws {
        _ = try await send("PUT", path, body: try JSONEncoder().encode(body))
    }

    func delete(_ path: String, query: [String: String] = [:]) async throws {
        _ = try await send("DELETE", path, query: query)
    }

    /// GET that maps 404 (path absent from config) to nil.
    func getOptional<T: Decodable>(_ path: String, as type: T.Type = T.self) async throws -> T? {
        do {
            return try await get(path, as: T.self)
        } catch APIError.status(404, _) {
            return nil
        }
    }

    func isHealthy() async -> Bool {
        (try? await send("GET", "/v8/management/config/server")) != nil
    }

    private func send(_ method: String, _ path: String, query: [String: String] = [:], body: Data? = nil) async throws -> Data {
        var components = URLComponents(url: baseURL.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
        if !query.isEmpty {
            components.queryItems = query.sorted { $0.key < $1.key }.map { URLQueryItem(name: $0.key, value: $0.value) }
        }
        var request = URLRequest(url: components.url!)
        request.httpMethod = method
        request.setValue("Bearer \(password)", forHTTPHeaderField: "Authorization")
        if let body {
            request.httpBody = body
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        let (data, response) = try await Self.session.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard (200..<300).contains(status) else {
            let message = (try? JSONDecoder().decode(ErrorBody.self, from: data))?.text
                ?? String(decoding: data, as: UTF8.self)
            throw APIError.status(status, message)
        }
        return data
    }

    private struct ErrorBody: Decodable {
        let error: String?
        let message: String?
        var text: String? { message ?? error }
    }
}

enum APIError: LocalizedError {
    case status(Int, String)
    case decoding(String, Error)

    var errorDescription: String? {
        switch self {
        case let .status(code, message): return "Engine returned \(code): \(message)"
        case let .decoding(path, error): return "Unexpected response from \(path): \(error.localizedDescription)"
        }
    }
}

// MARK: - Response models

struct AuthURLResponse: Decodable {
    let url: String
    let state: String
}

struct AuthStatusResponse: Decodable {
    let status: String
    let error: String?
}

struct CredentialList: Decodable {
    let files: [Credential]
}

struct Credential: Decodable, Identifiable, Equatable {
    let id: String
    let name: String
    let provider: String
    let email: String?
    let label: String?
    let status: String?
    let statusMessage: String?
    let disabled: Bool?
    let unavailable: Bool?
    let idToken: IDTokenClaims?

    struct IDTokenClaims: Decodable, Equatable {
        let planType: String?
        enum CodingKeys: String, CodingKey { case planType = "plan_type" }
    }

    enum CodingKeys: String, CodingKey {
        case id, name, provider, email, label, status, disabled, unavailable
        case statusMessage = "status_message"
        case idToken = "id_token"
    }

    var displayName: String {
        if let email, !email.isEmpty { return email }
        if let label, !label.isEmpty { return label }
        return name
    }

    var planLabel: String? {
        guard let plan = idToken?.planType, !plan.isEmpty else { return nil }
        return plan.prefix(1).uppercased() + plan.dropFirst()
    }

    var isHealthy: Bool { disabled != true && unavailable != true }
}

struct ModelDefinitionList: Decodable {
    let models: [ModelDefinition]
}

struct ModelDefinition: Decodable, Identifiable, Hashable {
    let id: String
    let displayName: String?
    let contextLength: Int?

    enum CodingKeys: String, CodingKey {
        case id
        case displayName = "display_name"
        case contextLength = "context_length"
    }
}

struct ModelAlias: Codable, Hashable {
    var name: String
    var alias: String
    var fork: Bool?
}

struct ClaudeUpstreamSettings: Decodable {
    let disableCloakMode: Bool?
    let disableCloakingModelList: Bool?

    enum CodingKeys: String, CodingKey {
        case disableCloakMode = "disable-claude-cloak-mode"
        case disableCloakingModelList = "disable-cloaking-model-list"
    }
}

struct CodexUpstreamSettings: Decodable {
    let disableCloaking: Bool?

    enum CodingKeys: String, CodingKey {
        case disableCloaking = "disable-codex-cloaking"
    }
}

struct UsageSummary: Decodable {
    let total: Point

    struct Point: Decodable {
        let requests: Int
        let failures: Int
        let tokens: Tokens
        let costUSD: Double?

        enum CodingKeys: String, CodingKey {
            case requests, failures, tokens
            case costUSD = "cost_usd"
        }
    }

    struct Tokens: Decodable {
        let total: Int
    }
}
