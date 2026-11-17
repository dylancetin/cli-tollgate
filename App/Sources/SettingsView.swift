import AppKit
import SwiftUI

struct SettingsView: View {
    @EnvironmentObject private var model: AppModel
    @EnvironmentObject private var updater: Updater
    @State private var portDraft = ""
    @State private var confirmRegenerate = false
    @State private var grantToRevoke: AccessGrant?
    // Web access edits. Toggles and pickers save at once; API keys and the site
    // list save on Return or with the Save button, not on every keystroke.
    @State private var webDraft = WebToolsConfig()
    @State private var webDomainsText = ""
    @State private var webLastSaved: WebToolsConfig?

    private struct WebProviderInfo: Identifiable {
        let id: String
        let title: String
        let keyURL: URL
    }

    private static let webProviders: [WebProviderInfo] = [
        WebProviderInfo(id: "brave", title: "Brave Search", keyURL: URL(string: "https://brave.com/search/api/")!),
        WebProviderInfo(id: "tavily", title: "Tavily", keyURL: URL(string: "https://app.tavily.com/")!),
        WebProviderInfo(id: "exa", title: "Exa", keyURL: URL(string: "https://dashboard.exa.ai/api-keys")!),
    ]

    var body: some View {
        Form {
            Section("General") {
                Toggle("Launch at login", isOn: Binding(
                    get: { model.launchAtLogin },
                    set: { model.setLaunchAtLogin($0) }
                ))
            }

            Section {
                LabeledContent("Port") {
                    HStack {
                        TextField("", text: $portDraft)
                            .frame(width: 70)
                            .multilineTextAlignment(.trailing)
                            .onSubmit(applyPort)
                        Button("Apply", action: applyPort)
                            .disabled(Int(portDraft) == model.port || Int(portDraft) == nil)
                    }
                }
                LabeledContent("Listens on") {
                    Text("127.0.0.1 only").foregroundStyle(.secondary)
                }
                LabeledContent("API key") {
                    Button("Regenerate…") { confirmRegenerate = true }
                }
            } header: {
                Text("Proxy")
            } footer: {
                Text("Web pages can call the proxy (CORS is open), but every request needs the API key.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section {
                if model.grants.isEmpty {
                    Text("No apps yet. When a web app asks for access, CLI Tollgate asks you first.")
                        .foregroundStyle(.secondary)
                }
                ForEach(model.grants) { grant in
                    HStack {
                        Image(systemName: "key")
                            .foregroundStyle(.secondary)
                        VStack(alignment: .leading, spacing: 2) {
                            HStack(spacing: 4) {
                                Text(grant.name).lineLimit(1)
                                if grant.hasWeb {
                                    Text("Web")
                                        .font(.caption2.weight(.semibold))
                                        .padding(.horizontal, 5)
                                        .padding(.vertical, 1)
                                        .background(Color.accentColor.opacity(0.15), in: Capsule())
                                }
                            }
                            Text(grantCaption(grant))
                                .font(.caption)
                                .foregroundStyle(.secondary)
                                .lineLimit(1)
                                .truncationMode(.middle)
                        }
                        Spacer()
                        Button("Revoke…") { grantToRevoke = grant }
                    }
                }
            } header: {
                Text("Connected apps")
            } footer: {
                Text("Revoking an app turns off its key right away. Regenerating your API key leaves connected apps working.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .disabled(model.engineState != .running)

            webAccessSection

            Section("Updates") {
                Toggle("Check for updates automatically", isOn: $updater.automaticallyChecks)
                Toggle("Download and install updates automatically", isOn: $updater.automaticallyDownloads)
                    .disabled(!updater.automaticallyChecks)
                Button("Check Now") { updater.checkForUpdates() }
                    .disabled(!updater.canCheckForUpdates)
            }

            Section {
                Toggle(isOn: Binding(
                    get: { model.cloakingEnabled },
                    set: { on in Task { await model.setCloaking(on) } }
                )) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Cloak requests")
                        Text(cloakingCaption)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                Toggle("Disguise Claude requests as Claude Code", isOn: Binding(
                    get: { model.claudeCloaking },
                    set: { on in Task { await model.setClaudeCloaking(on) } }
                ))
                Toggle("Send official Codex CLI headers for GPT / Codex", isOn: Binding(
                    get: { model.codexCloaking },
                    set: { on in Task { await model.setCodexCloaking(on) } }
                ))
                Toggle("Cloak Claude model IDs in /v1/models", isOn: Binding(
                    get: { model.claudeCloakModelList },
                    set: { on in Task { await model.setClaudeCloakModelList(on) } }
                ))
            } header: {
                Text("Advanced: Cloaking")
            } footer: {
                Text("The disguise follows the requested model. Requests that already come from Claude Code are passed through unchanged.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .disabled(model.engineState != .running)

            Section("About") {
                LabeledContent("Version", value: appVersion)
                LabeledContent("Engine", value: engineCommit)
                Button("Show Data Folder") { NSWorkspace.shared.activateFileViewerSelecting([Paths.config]) }
            }
        }
        .formStyle(.grouped)
        // The form scrolls inside a resizable window instead of growing to
        // its full content height, which ran past the bottom of the screen.
        .frame(width: 460)
        .frame(minHeight: 320, idealHeight: 620, maxHeight: .infinity)
        .onAppear {
            portDraft = String(model.port)
            loadWebDraft(model.webTools)
            Task { await model.refreshGrants() }
        }
        .onChange(of: model.port) { portDraft = String($0) }
        .onChange(of: model.webTools) { tools in
            // Our own save comes back as the same value; anything else (a refresh
            // after a failed save, say) replaces what is on screen.
            if tools != webLastSaved { loadWebDraft(tools) }
        }
        .confirmationDialog("Regenerate the API key?", isPresented: $confirmRegenerate) {
            Button("Regenerate", role: .destructive) {
                Task {
                    do { try await model.regenerateAPIKey() } catch { model.lastError = error.localizedDescription }
                }
            }
        } message: {
            Text("Clients using the current key will stop working until you update them. Connected apps keep their own keys.")
        }
        .confirmationDialog(
            "Revoke access for “\(grantToRevoke?.name ?? "")”?",
            isPresented: Binding(get: { grantToRevoke != nil }, set: { if !$0 { grantToRevoke = nil } }),
            presenting: grantToRevoke
        ) { grant in
            Button("Revoke", role: .destructive) {
                Task { await model.revokeGrant(grant) }
            }
        } message: { _ in
            Text("Its key stops working right away. It can ask for access again later.")
        }
    }

    private var webAccessSection: some View {
        Section {
            Toggle("Let connected apps search and read the web", isOn: Binding(
                get: { webDraft.enabled },
                set: { on in
                    webDraft.enabled = on
                    saveWeb()
                }
            ))
            ForEach(Self.webProviders) { provider in
                LabeledContent(provider.title) {
                    HStack {
                        SecureField("", text: Binding(
                            get: { webDraft.apiKey(for: provider.id) },
                            set: { webDraft.setAPIKey($0, for: provider.id) }
                        ))
                        .frame(width: 170)
                        .onSubmit(saveWeb)
                        Link("Get a key", destination: provider.keyURL)
                    }
                }
            }
            LabeledContent("Default search") {
                Picker("", selection: Binding(
                    get: { webPending.search.defaultProvider },
                    set: { id in
                        webDraft.search.defaultProvider = id
                        saveWeb()
                    }
                )) {
                    if webPending.providersWithKeys.isEmpty {
                        Text("—").tag(webPending.search.defaultProvider)
                    }
                    ForEach(webProvidersWithKeys) { provider in
                        Text(provider.title).tag(provider.id)
                    }
                }
                .labelsHidden()
                .disabled(webPending.providersWithKeys.isEmpty)
            }
            if webPending.providersWithKeys.isEmpty {
                Text("Add an API key to enable search. Reading pages works without one.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Picker("Site list", selection: Binding(
                get: { webDraft.domains.mode },
                set: { mode in
                    webDraft.domains.mode = mode
                    saveWeb()
                }
            )) {
                Text("Block listed sites").tag("denylist")
                Text("Allow only listed sites").tag("allowlist")
            }
            VStack(alignment: .leading, spacing: 4) {
                TextEditor(text: $webDomainsText)
                    .font(.system(.body, design: .monospaced))
                    .frame(minHeight: 70)
                Text("One site per line, e.g. example.com")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            HStack {
                Spacer()
                Button("Save", action: saveWeb)
                    .disabled(webPending == model.webTools)
            }
        } header: {
            Text("Web access")
        } footer: {
            Text("Searches and page reads go out from this computer, so sites see your IP address. A site entry also covers its subdomains.")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .disabled(model.engineState != .running)
    }

    /// The draft with the site list text applied, cleaned the way it will be saved.
    private var webPending: WebToolsConfig {
        var config = webDraft
        config.domains.list = WebToolsConfig.domains(fromText: webDomainsText)
        return config.normalized()
    }

    private var webProvidersWithKeys: [WebProviderInfo] {
        let withKeys = webPending.providersWithKeys
        return Self.webProviders.filter { withKeys.contains($0.id) }
    }

    private func loadWebDraft(_ config: WebToolsConfig) {
        webDraft = config
        webDomainsText = config.domains.list.joined(separator: "\n")
    }

    private func saveWeb() {
        let config = webPending
        webLastSaved = config
        Task { await model.setWebTools(config) }
    }

    private var cloakingCaption: String {
        switch (model.claudeCloaking, model.codexCloaking) {
        case (true, true): return String(localized: "Claude models go out as Claude Code, GPT / Codex as Codex CLI.")
        case (false, false): return String(localized: "Requests keep the calling client's identity.")
        default: return String(localized: "Partially on. Adjust each side below.")
        }
    }

    private func grantCaption(_ grant: AccessGrant) -> String {
        var parts = [grant.origin]
        if let date = grant.grantedDate {
            let day = date.formatted(date: .abbreviated, time: .omitted)
            parts.append(String(localized: "allowed \(day)"))
        }
        parts.append(grant.keyHint)
        return parts.joined(separator: " · ")
    }

    private func applyPort() {
        guard let port = Int(portDraft) else { return }
        Task { await model.changePort(to: port) }
    }

    private var appVersion: String {
        let info = Bundle.main.infoDictionary
        let short = info?["CFBundleShortVersionString"] as? String ?? "?"
        let build = info?["CFBundleVersion"] as? String ?? "?"
        return "\(short) (\(build))"
    }

    private var engineCommit: String {
        guard let url = Bundle.main.url(forResource: "engine-commit", withExtension: "txt"),
              let text = try? String(contentsOf: url, encoding: .utf8) else { return String(localized: "bundled") }
        return "CLIProxyAPI @ " + String(text.trimmingCharacters(in: .whitespacesAndNewlines).prefix(12))
    }
}
