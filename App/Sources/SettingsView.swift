import AppKit
import SwiftUI

struct SettingsView: View {
    @EnvironmentObject private var model: AppModel
    @EnvironmentObject private var updater: Updater
    @State private var portDraft = ""
    @State private var confirmRegenerate = false
    @State private var grantToRevoke: AccessGrant?

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
                            Text(grant.name).lineLimit(1)
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
            Task { await model.refreshGrants() }
        }
        .onChange(of: model.port) { portDraft = String($0) }
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

    private var cloakingCaption: String {
        switch (model.claudeCloaking, model.codexCloaking) {
        case (true, true): return "Claude models go out as Claude Code, GPT / Codex as Codex CLI."
        case (false, false): return "Requests keep the calling client's identity."
        default: return "Partially on. Adjust each side below."
        }
    }

    private func grantCaption(_ grant: AccessGrant) -> String {
        var parts = [grant.origin]
        if let date = grant.grantedDate {
            parts.append("allowed " + date.formatted(date: .abbreviated, time: .omitted))
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
              let text = try? String(contentsOf: url, encoding: .utf8) else { return "bundled" }
        return "CLIProxyAPI @ " + String(text.trimmingCharacters(in: .whitespacesAndNewlines).prefix(12))
    }
}
