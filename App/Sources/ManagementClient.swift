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

    /// Long polls (access requests) hold the request open for up to ~55s.
    private static let longPollSession: URLSession = {
        let config = URLSessionConfiguration.ephemeral
        config.timeoutIntervalForRequest = 70
        config.connectionProxyDictionary = [:]
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

    /// GET for endpoints that wait (`?wait=`) before answering.
    func longGet<T: Decodable>(_ path: String, query: [String: String] = [:], as type: T.Type = T.self) async throws -> T {
        let data = try await send("GET", path, query: query, session: Self.longPollSession)
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw APIError.decoding(path, error)
        }
    }

    func post(_ path: String) async throws {
        _ = try await send("POST", path)
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

    private func send(_ method: String, _ path: String, query: [String: String] = [:], body: Data? = nil,
                      session: URLSession = ManagementClient.session) async throws -> Data {
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
        let (data, response) = try await session.data(for: request)
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
        case let .status(code, message): return String(localized: "Engine returned \(code): \(message)")
        case let .decoding(path, error):
            return String(localized: "Unexpected response from \(path): \(error.localizedDescription)")
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

// MARK: - Access requests

/// A local app or web page asking for an API key (engine: POST /access/requests).
struct AccessRequest: Decodable, Identifiable, Equatable {
    let id: String
    let name: String
    let origin: String
    let createdAt: String?
    /// What the app asks for. Older engines send nothing, which means ["models"].
    let scopes: [String]
    /// True when an already connected app asks for more scopes on its existing grant.
    let upgrade: Bool
    let grantID: String?
    let addedScopes: [String]

    enum CodingKeys: String, CodingKey {
        case id, name, origin, scopes, upgrade
        case createdAt = "created_at"
        case grantID = "grant_id"
        case addedScopes = "added_scopes"
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        origin = try container.decode(String.self, forKey: .origin)
        createdAt = try container.decodeIfPresent(String.self, forKey: .createdAt)
        let decodedScopes = try container.decodeIfPresent([String].self, forKey: .scopes) ?? []
        scopes = decodedScopes.isEmpty ? ["models"] : decodedScopes
        upgrade = try container.decodeIfPresent(Bool.self, forKey: .upgrade) ?? false
        grantID = try container.decodeIfPresent(String.self, forKey: .grantID)
        addedScopes = try container.decodeIfPresent([String].self, forKey: .addedScopes) ?? []
    }

    /// A new connection that also asks for web access.
    var asksWebOnConnect: Bool { !upgrade && scopes.contains("web") }
    /// An existing app asking to also use the web tools.
    var isWebUpgrade: Bool { upgrade && addedScopes.contains("web") }

    /// What to tell the user about the origin; `warn` marks the risky cases.
    var originNote: (text: String, warn: Bool)? {
        if origin == "local program" { return (String(localized: "A program on this computer, not a web page."), true) }
        let local = ["http://localhost", "http://127.0.0.1", "http://[::1]"]
        if local.contains(where: { origin == $0 || origin.hasPrefix($0 + ":") }) {
            return (String(localized: "A page served from this computer."), false)
        }
        if origin.hasPrefix("http://") { return (String(localized: "Not a secure (https) site."), true) }
        return nil
    }
}

struct AccessRequestList: Decodable {
    let requests: [AccessRequest]
}

/// An app that was allowed in. Its key lives in access.api-keys.
struct AccessGrant: Decodable, Identifiable, Equatable {
    let id: String
    let name: String
    let origin: String
    let keyHint: String
    let grantedAt: String
    /// Missing in answers from older engines, which means ["models"].
    let scopes: [String]

    enum CodingKeys: String, CodingKey {
        case id, name, origin, scopes
        case keyHint = "key_hint"
        case grantedAt = "granted_at"
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        origin = try container.decode(String.self, forKey: .origin)
        keyHint = try container.decode(String.self, forKey: .keyHint)
        grantedAt = try container.decode(String.self, forKey: .grantedAt)
        let decodedScopes = try container.decodeIfPresent([String].self, forKey: .scopes) ?? []
        scopes = decodedScopes.isEmpty ? ["models"] : decodedScopes
    }

    var hasWeb: Bool { scopes.contains("web") }

    var grantedDate: Date? {
        let withFraction = ISO8601DateFormatter()
        withFraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return withFraction.date(from: grantedAt) ?? ISO8601DateFormatter().date(from: grantedAt)
    }
}

struct AccessGrantList: Decodable {
    let grants: [AccessGrant]
}

// MARK: - Web access

/// The engine's `web-tools` config section. It is always sent whole.
struct WebToolsConfig: Codable, Equatable {
    /// Search providers in the order they are offered.
    static let providerIDs = ["brave", "tavily", "exa", "parallel", "firecrawl", "tinyfish"]

    /// Providers that search without an API key (with lower limits). Brave has no keyless mode.
    static let keylessProviderIDs: Set<String> = ["tavily", "exa", "parallel", "firecrawl", "tinyfish"]

    /// The default that lets the engine pick a provider and move on when one is rate limited.
    static let autoProvider = "auto"

    struct ProviderEntry: Codable, Equatable {
        var apiKey: String

        enum CodingKeys: String, CodingKey { case apiKey = "api-key" }

        init(apiKey: String = "") { self.apiKey = apiKey }

        init(from decoder: Decoder) throws {
            let container = try decoder.container(keyedBy: CodingKeys.self)
            apiKey = try container.decodeIfPresent(String.self, forKey: .apiKey) ?? ""
        }
    }

    struct Search: Codable, Equatable {
        /// "auto" or a provider ID.
        var defaultProvider: String
        /// Lets providers that support it search without an API key.
        var keyless: Bool
        var providers: [String: ProviderEntry]

        enum CodingKeys: String, CodingKey {
            case defaultProvider = "default"
            case keyless
            case providers
        }

        init(defaultProvider: String = WebToolsConfig.autoProvider, keyless: Bool = true, providers: [String: ProviderEntry] = [:]) {
            self.defaultProvider = defaultProvider
            self.keyless = keyless
            self.providers = providers
        }

        init(from decoder: Decoder) throws {
            let container = try decoder.container(keyedBy: CodingKeys.self)
            defaultProvider = try container.decodeIfPresent(String.self, forKey: .defaultProvider) ?? WebToolsConfig.autoProvider
            keyless = try container.decodeIfPresent(Bool.self, forKey: .keyless) ?? true
            providers = try container.decodeIfPresent([String: ProviderEntry].self, forKey: .providers) ?? [:]
        }
    }

    struct Domains: Codable, Equatable {
        /// "denylist" or "allowlist".
        var mode: String
        var list: [String]

        enum CodingKeys: String, CodingKey { case mode, list }

        init(mode: String = "denylist", list: [String] = []) {
            self.mode = mode
            self.list = list
        }

        init(from decoder: Decoder) throws {
            let container = try decoder.container(keyedBy: CodingKeys.self)
            mode = try container.decodeIfPresent(String.self, forKey: .mode) ?? "denylist"
            list = try container.decodeIfPresent([String].self, forKey: .list) ?? []
        }
    }

    var enabled: Bool
    var search: Search
    var domains: Domains

    enum CodingKeys: String, CodingKey { case enabled, search, domains }

    /// What an engine without a `web-tools` section behaves like.
    init() {
        enabled = true
        search = Search()
        domains = Domains()
        normalize()
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        enabled = try container.decodeIfPresent(Bool.self, forKey: .enabled) ?? true
        search = try container.decodeIfPresent(Search.self, forKey: .search) ?? Search()
        domains = try container.decodeIfPresent(Domains.self, forKey: .domains) ?? Domains()
        normalize()
    }

    /// Whether a provider can search: it has an API key, or it works without
    /// one and keyless search is on.
    func canSearch(_ provider: String) -> Bool {
        guard Self.providerIDs.contains(provider) else { return false }
        let key = apiKey(for: provider).trimmingCharacters(in: .whitespacesAndNewlines)
        return !key.isEmpty || (search.keyless && Self.keylessProviderIDs.contains(provider))
    }

    /// Providers that can search, in the order they are offered.
    var usableProviders: [String] {
        Self.providerIDs.filter { canSearch($0) }
    }

    func apiKey(for provider: String) -> String { search.providers[provider]?.apiKey ?? "" }

    mutating func setAPIKey(_ key: String, for provider: String) {
        search.providers[provider] = ProviderEntry(apiKey: key)
    }

    /// Trims keys, fills in every provider, keeps the default on "auto" or a
    /// provider that can search, and cleans the site list (one hostname each,
    /// lowercase, no repeats).
    mutating func normalize() {
        for id in Self.providerIDs {
            search.providers[id] = ProviderEntry(apiKey: apiKey(for: id).trimmingCharacters(in: .whitespacesAndNewlines))
        }
        // The engine treats any other default as "auto", so store it that way.
        if search.defaultProvider != Self.autoProvider && !canSearch(search.defaultProvider) {
            search.defaultProvider = Self.autoProvider
        }
        if domains.mode != "allowlist" { domains.mode = "denylist" }
        domains.list = Self.cleanDomains(domains.list)
    }

    func normalized() -> WebToolsConfig {
        var copy = self
        copy.normalize()
        return copy
    }

    static func cleanDomains(_ list: [String]) -> [String] {
        var seen = Set<String>()
        var out: [String] = []
        for entry in list {
            let host = entry.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
            if host.isEmpty || seen.contains(host) { continue }
            seen.insert(host)
            out.append(host)
        }
        return out
    }

    /// One hostname per line, as typed in Settings.
    static func domains(fromText text: String) -> [String] {
        cleanDomains(text.components(separatedBy: .newlines))
    }
}
