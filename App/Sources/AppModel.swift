import AppKit
import Foundation
import ServiceManagement

enum Provider: String, CaseIterable, Identifiable {
    case codex
    case claude

    var id: String { rawValue }

    var title: String {
        switch self {
        case .codex: return "ChatGPT"
        case .claude: return "Claude"
        }
    }

    var subtitle: String {
        switch self {
        case .codex: return String(localized: "Plus / Pro subscription via Codex")
        case .claude: return String(localized: "Pro / Max subscription")
        }
    }

    var symbol: String {
        switch self {
        case .codex: return "bubble.left.and.text.bubble.right"
        case .claude: return "sparkle"
        }
    }

    /// Provider value used by the engine's OAuth endpoint and credential files.
    var engineProvider: String { rawValue }
    /// Channel name used by model definitions, aliases and exclusions.
    var channel: String { rawValue }
}

@MainActor
final class AppModel: ObservableObject {
    static let shared = AppModel()

    let engine = EngineController()

    @Published private(set) var engineState: EngineController.State = .stopped
    @Published private(set) var port: Int = EngineConfig.defaultPort
    @Published private(set) var apiKey: String = ""

    @Published private(set) var accounts: [Provider: Credential] = [:]
    @Published private(set) var loginInProgress: Provider?
    @Published var lastError: String?

    @Published private(set) var catalog: [Provider: [ModelDefinition]] = [:]
    @Published private(set) var excluded: [Provider: Set<String>] = [:]
    @Published private(set) var aliases: [Provider: [ModelAlias]] = [:]

    @Published private(set) var claudeCloaking = true
    @Published private(set) var claudeCloakModelList = true
    @Published private(set) var codexCloaking = true

    @Published private(set) var usageToday: UsageSummary.Point?
    @Published private(set) var launchAtLogin = SMAppService.mainApp.status == .enabled

    @Published private(set) var grants: [AccessGrant] = []

    private var loginState: String?
    private var refreshTimer: Timer?
    private var accessWatcher: Task<Void, Never>?

    private var client: ManagementClient { engine.client }

    var baseURL: String { "http://127.0.0.1:\(port)" }

    var menuBarSymbol: String {
        switch engineState {
        case .running: return accounts.isEmpty ? "door.left.hand.closed" : "door.left.hand.open"
        case .starting: return "hourglass"
        case .stopped: return "door.left.hand.closed"
        case .failed: return "exclamationmark.triangle"
        }
    }

    private init() {
        engine.onStateChange = { [weak self] state in
            guard let self else { return }
            self.engineState = state
            self.port = self.engine.port
            if state == .running { Task { await self.refreshAll() } }
        }
    }

    func start() {
        engine.start()
        if accessWatcher == nil {
            accessWatcher = Task { await self.watchAccessRequests() }
        }
        refreshTimer?.invalidate()
        refreshTimer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            Task { @MainActor in
                guard let self, self.engineState == .running else { return }
                await self.refreshAccounts()
                await self.refreshUsage()
            }
        }
    }

    func setEngineEnabled(_ enabled: Bool) {
        if enabled { engine.start() } else { engine.stop() }
    }

    func restartEngine() { engine.restart() }

    // MARK: - Refresh

    func refreshAll() async {
        await refreshAccounts()
        await refreshSettings()
        await refreshModels()
        await refreshUsage()
        await refreshGrants()
    }

    func refreshAccounts() async {
        guard engineState == .running else { return }
        do {
            let list: CredentialList = try await client.get("/v8/management/credentials")
            var next: [Provider: Credential] = [:]
            for provider in Provider.allCases {
                // One account per provider: prefer a healthy one if several exist.
                let matches = list.files.filter { $0.provider.lowercased() == provider.engineProvider }
                next[provider] = matches.first(where: \.isHealthy) ?? matches.first
            }
            accounts = next
        } catch {
            report(error)
        }
    }

    func refreshSettings() async {
        guard engineState == .running else { return }
        do {
            let keys: [String]? = try await client.getOptional("/v8/management/config/access/api-keys")
            if let first = keys?.first {
                apiKey = first
            } else {
                // Never leave the proxy unauthenticated.
                try await regenerateAPIKey()
            }
            if let claude: ClaudeUpstreamSettings = try await client.getOptional("/v8/management/config/upstream/claude") {
                claudeCloaking = !(claude.disableCloakMode ?? false)
                claudeCloakModelList = !(claude.disableCloakingModelList ?? false)
            }
            if let codex: CodexUpstreamSettings = try await client.getOptional("/v8/management/config/upstream/codex") {
                codexCloaking = !(codex.disableCloaking ?? false)
            }
        } catch {
            report(error)
        }
    }

    func refreshModels() async {
        guard engineState == .running else { return }
        do {
            for provider in Provider.allCases {
                let defs: ModelDefinitionList = try await client.get("/v8/management/routing/model-definitions/\(provider.channel)")
                catalog[provider] = defs.models
            }
            let ex: [String: [String]]? = try await client.getOptional("/v8/management/config/oauth/excluded-models")
            let al: [String: [ModelAlias]]? = try await client.getOptional("/v8/management/config/oauth/model-alias")
            for provider in Provider.allCases {
                excluded[provider] = Set(ex?[provider.channel] ?? [])
                aliases[provider] = al?[provider.channel] ?? []
            }
        } catch {
            report(error)
        }
    }

    func refreshUsage() async {
        let startOfDay = Calendar.current.startOfDay(for: Date())
        guard engineState == .running else { return }
        let summary = try? await client.get(
            "/v8/management/observability/usage/summary",
            query: ["from": String(Int(startOfDay.timeIntervalSince1970))],
            as: UsageSummary.self
        )
        usageToday = summary?.total
    }

    // MARK: - Accounts

    func login(_ provider: Provider) async {
        guard loginInProgress == nil else { return }
        loginInProgress = provider
        lastError = nil
        defer {
            loginInProgress = nil
            loginState = nil
        }
        do {
            let before = try await credentialNames(for: provider)
            let start: AuthURLResponse = try await client.get(
                "/v8/management/oauth/auth-url",
                query: ["provider": provider.engineProvider, "is_webui": "true"]
            )
            loginState = start.state
            guard let url = URL(string: start.url) else { throw EngineError(String(localized: "Invalid login URL.")) }
            NSWorkspace.shared.open(url)

            let deadline = Date().addingTimeInterval(300)
            while true {
                try await Task.sleep(nanoseconds: 1_500_000_000)
                guard loginState == start.state else { return } // cancelled
                guard Date() < deadline else { throw EngineError(String(localized: "Login timed out.")) }
                let status: AuthStatusResponse = try await client.get(
                    "/v8/management/oauth/status", query: ["state": start.state]
                )
                if status.status == "ok" { break }
                if status.status == "error" { throw EngineError(status.error ?? String(localized: "Login failed.")) }
            }

            // Keep exactly one account per provider: drop the ones that existed before.
            var added: Set<String> = []
            for _ in 0..<10 {
                added = try await credentialNames(for: provider).subtracting(before)
                if !added.isEmpty { break }
                try await Task.sleep(nanoseconds: 500_000_000)
            }
            if !added.isEmpty {
                for name in before.subtracting(added) {
                    try? await client.delete("/v8/management/credentials", query: ["name": name])
                }
            }
            await refreshAccounts()
            NSApp.activate(ignoringOtherApps: true)
        } catch {
            report(error)
        }
    }

    func cancelLogin() {
        guard let state = loginState else { return }
        loginState = nil
        Task { try? await client.delete("/v8/management/oauth/session", query: ["state": state]) }
    }

    func logout(_ provider: Provider) async {
        do {
            for name in try await credentialNames(for: provider) {
                try await client.delete("/v8/management/credentials", query: ["name": name])
            }
            await refreshAccounts()
        } catch {
            report(error)
        }
    }

    private func credentialNames(for provider: Provider) async throws -> Set<String> {
        let list: CredentialList = try await client.get("/v8/management/credentials")
        return Set(list.files.filter { $0.provider.lowercased() == provider.engineProvider }.map(\.name))
    }

    // MARK: - Models

    func isExposed(_ model: String, for provider: Provider) -> Bool {
        !(excluded[provider]?.contains(model) ?? false)
    }

    func setExposed(_ exposed: Bool, model: String, for provider: Provider) async {
        var set = excluded[provider] ?? []
        if exposed { set.remove(model) } else { set.insert(model) }
        excluded[provider] = set
        await save("/v8/management/config/oauth/excluded-models/\(provider.channel)", set.sorted())
    }

    func setAllExposed(_ exposed: Bool, for provider: Provider) async {
        let set: Set<String> = exposed ? [] : Set((catalog[provider] ?? []).map(\.id))
        excluded[provider] = set
        await save("/v8/management/config/oauth/excluded-models/\(provider.channel)", set.sorted())
    }

    func alias(for model: String, provider: Provider) -> String {
        aliases[provider]?.first(where: { $0.name == model })?.alias ?? ""
    }

    /// Exposes `model` under an extra client-visible name (the original stays available).
    func setAlias(_ alias: String, model: String, for provider: Provider) async {
        let trimmed = alias.trimmingCharacters(in: .whitespacesAndNewlines)
        var list = (aliases[provider] ?? []).filter { $0.name != model }
        if !trimmed.isEmpty {
            list.append(ModelAlias(name: model, alias: trimmed, fork: true))
        }
        aliases[provider] = list
        await save("/v8/management/config/oauth/model-alias/\(provider.channel)", list)
    }

    // MARK: - Cloaking
    //
    // The engine picks the disguise from the model: Claude models go to the Claude
    // executor (Claude Code identity), GPT/Codex models go to the Codex executor
    // (Codex CLI identity). The master switch turns both on or off together.

    var cloakingEnabled: Bool { claudeCloaking && codexCloaking }

    func setCloaking(_ on: Bool) async {
        claudeCloaking = on
        codexCloaking = on
        await save("/v8/management/config/upstream/claude/disable-claude-cloak-mode", !on)
        await save("/v8/management/config/upstream/codex/disable-codex-cloaking", !on)
    }

    func setClaudeCloaking(_ on: Bool) async {
        claudeCloaking = on
        await save("/v8/management/config/upstream/claude/disable-claude-cloak-mode", !on)
    }

    func setClaudeCloakModelList(_ on: Bool) async {
        claudeCloakModelList = on
        await save("/v8/management/config/upstream/claude/disable-cloaking-model-list", !on)
    }

    func setCodexCloaking(_ on: Bool) async {
        codexCloaking = on
        await save("/v8/management/config/upstream/codex/disable-codex-cloaking", !on)
    }

    // MARK: - Settings

    /// Replaces the app's own key (the first one). Keys handed to apps through
    /// access requests stay valid.
    func regenerateAPIKey() async throws {
        let key = EngineConfig.generateAPIKey()
        let keys: [String] = try await client.getOptional("/v8/management/config/access/api-keys") ?? []
        try await client.put("/v8/management/config/access/api-keys", [key] + Array(keys.dropFirst()))
        apiKey = key
    }

    func changePort(to newPort: Int) async {
        guard (1024...65535).contains(newPort), newPort != port else { return }
        do {
            try await client.put("/v8/management/config/server/port", newPort)
            engine.restart()
        } catch {
            report(error)
        }
    }

    func setLaunchAtLogin(_ enabled: Bool) {
        do {
            if enabled { try SMAppService.mainApp.register() } else { try SMAppService.mainApp.unregister() }
        } catch {
            report(error)
        }
        launchAtLogin = SMAppService.mainApp.status == .enabled
    }

    // MARK: - Access requests
    //
    // A web app generates its own key and calls the engine's POST /access/requests.
    // We long-poll the pending list (which is also what lets the engine accept
    // requests at all), ask the user, and approve or deny through the management API.

    private func watchAccessRequests() async {
        while !Task.isCancelled {
            guard engineState == .running else {
                try? await Task.sleep(nanoseconds: 2_000_000_000)
                continue
            }
            do {
                let list: AccessRequestList = try await client.longGet(
                    "/v8/management/access/requests", query: ["wait": "50"]
                )
                for request in list.requests {
                    await prompt(for: request)
                }
            } catch APIError.status(404, _) {
                try? await Task.sleep(nanoseconds: 60_000_000_000) // engine without access requests
            } catch {
                try? await Task.sleep(nanoseconds: 3_000_000_000)
            }
        }
    }

    private func prompt(for request: AccessRequest) async {
        let alert = NSAlert()
        alert.messageText = String(localized: "Allow “\(request.name)” to use your subscriptions?")
        var info = request.origin
        if let note = request.originNote {
            info += "\n" + (note.warn ? "⚠︎ " : "") + note.text
        }
        info += "\n\n" + String(localized: "It will be able to send requests through your signed-in ChatGPT and Claude accounts. You can revoke access anytime in Settings.")
        alert.informativeText = info
        alert.alertStyle = request.originNote?.warn == true ? .warning : .informational
        // Deny is the default button so a stray Return never grants access.
        let deny = alert.addButton(withTitle: String(localized: "Deny"))
        deny.keyEquivalent = "\r"
        let allow = alert.addButton(withTitle: String(localized: "Allow"))
        allow.keyEquivalent = ""
        alert.window.level = .floating
        NSApp.activate(ignoringOtherApps: true)
        let allowed = alert.runModal() == .alertSecondButtonReturn

        let verb = allowed ? "approve" : "deny"
        do {
            try await client.post("/v8/management/access/requests/\(request.id)/\(verb)")
            if allowed { await refreshGrants() }
        } catch APIError.status(404, _) {
            lastError = String(localized: "The request from \(request.name) expired before it was answered.")
        } catch {
            report(error)
        }
    }

    func refreshGrants() async {
        guard engineState == .running else { return }
        do {
            let list: AccessGrantList = try await client.get("/v8/management/access/grants")
            grants = list.grants
        } catch APIError.status(404, _) {
            grants = []
        } catch {
            report(error)
        }
    }

    func revokeGrant(_ grant: AccessGrant) async {
        do {
            try await client.delete("/v8/management/access/grants/\(grant.id)")
        } catch APIError.status(404, _) {
            // Already gone.
        } catch {
            report(error)
        }
        await refreshGrants()
    }

    // MARK: - Helpers

    private func save<Body: Encodable>(_ path: String, _ body: Body) async {
        do {
            try await client.put(path, body)
        } catch {
            report(error)
            await refreshSettings()
            await refreshModels()
        }
    }

    private func report(_ error: Error) {
        if error is CancellationError { return }
        lastError = error.localizedDescription
    }

    func copy(_ text: String) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }
}
