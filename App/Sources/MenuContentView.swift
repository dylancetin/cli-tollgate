import AppKit
import SwiftUI

struct MenuContentView: View {
    @EnvironmentObject private var model: AppModel
    @EnvironmentObject private var updater: Updater
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
                .padding(14)

            if let error = model.lastError {
                ErrorBanner(message: error) { model.lastError = nil }
                    .padding(.horizontal, 14)
                    .padding(.bottom, 10)
            }

            Divider()

            PanelSection(title: "Accounts") {
                ForEach(Provider.allCases) { provider in
                    AccountRow(provider: provider)
                }
            }

            Divider()

            PanelSection(title: "Connect") {
                ConnectPanel()
            }

            Divider()

            footer
                .padding(.horizontal, 10)
                .padding(.vertical, 8)
        }
        .frame(width: 340)
    }

    // MARK: - Header

    private var header: some View {
        HStack(spacing: 10) {
            ZStack {
                RoundedRectangle(cornerRadius: 8, style: .continuous)
                    .fill(Color.accentColor.gradient)
                Image(systemName: "door.left.hand.open")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(.white)
            }
            .frame(width: 32, height: 32)

            VStack(alignment: .leading, spacing: 2) {
                Text("CLI Tollgate").font(.headline)
                HStack(spacing: 5) {
                    Circle().fill(statusColor).frame(width: 7, height: 7)
                    Text(statusText)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                }
            }

            Spacer()

            Toggle("", isOn: Binding(
                get: { model.engineState == .running || model.engineState == .starting },
                set: { model.setEngineEnabled($0) }
            ))
            .toggleStyle(.switch)
            .labelsHidden()
            .controlSize(.small)
            .help("Start or stop the local proxy")
        }
    }

    private var statusColor: Color {
        switch model.engineState {
        case .running: return .green
        case .starting: return .orange
        case .stopped: return .secondary
        case .failed: return .red
        }
    }

    private var statusText: String {
        switch model.engineState {
        case .running: return "Running on \(model.baseURL)"
        case .starting: return "Starting…"
        case .stopped: return "Stopped"
        case let .failed(message): return message
        }
    }

    // MARK: - Footer

    private var footer: some View {
        HStack(spacing: 4) {
            if let usage = model.usageToday {
                Label("\(usage.requests) req · \(formatTokens(usage.tokens.total)) today", systemImage: "chart.bar")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .labelStyle(.titleAndIcon)
                    .help("\(usage.failures) failed requests today")
            }
            Spacer()
            FooterButton(symbol: "square.stack.3d.up", help: "Models") { show(WindowID.models) }
            FooterButton(symbol: "gearshape", help: "Settings") { show(WindowID.settings) }
            Menu {
                Button("Check for Updates…") { updater.checkForUpdates() }
                    .disabled(!updater.canCheckForUpdates)
                Button("Restart Engine") { model.restartEngine() }
                Button("Open Logs") { NSWorkspace.shared.open(Paths.engineLog) }
                Button("Show Data Folder") { NSWorkspace.shared.activateFileViewerSelecting([Paths.config]) }
                Divider()
                Button("Quit CLI Tollgate") { NSApp.terminate(nil) }
                    .keyboardShortcut("q")
            } label: {
                Image(systemName: "ellipsis.circle")
            }
            .menuStyle(.borderlessButton)
            .menuIndicator(.hidden)
            .fixedSize()
        }
    }

    private func show(_ id: String) {
        openWindow(id: id)
        NSApp.activate(ignoringOtherApps: true)
    }
}

func formatTokens(_ n: Int) -> String {
    switch n {
    case 1_000_000...: return String(format: "%.1fM tok", Double(n) / 1_000_000)
    case 1_000...: return String(format: "%.1fk tok", Double(n) / 1_000)
    default: return "\(n) tok"
    }
}

// MARK: - Building blocks

private struct PanelSection<Content: View>: View {
    let title: String
    @ViewBuilder var content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title.uppercased())
                .font(.caption2.weight(.semibold))
                .tracking(0.6)
                .foregroundStyle(.secondary)
            content
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
    }
}

private struct FooterButton: View {
    let symbol: String
    let help: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: symbol).frame(width: 22, height: 22)
        }
        .buttonStyle(.borderless)
        .help(help)
    }
}

private struct ErrorBanner: View {
    let message: String
    let dismiss: () -> Void

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(.orange)
            Text(message).font(.caption).textSelection(.enabled)
            Spacer(minLength: 0)
            Button(action: dismiss) { Image(systemName: "xmark") }
                .buttonStyle(.borderless)
                .font(.caption)
        }
        .padding(8)
        .background(Color.orange.opacity(0.12), in: RoundedRectangle(cornerRadius: 6))
    }
}

// MARK: - Accounts

private struct AccountRow: View {
    @EnvironmentObject private var model: AppModel
    let provider: Provider

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: provider.symbol)
                .font(.system(size: 13, weight: .medium))
                .frame(width: 26, height: 26)
                .background(Color.secondary.opacity(0.12), in: RoundedRectangle(cornerRadius: 6))

            VStack(alignment: .leading, spacing: 1) {
                HStack(spacing: 4) {
                    Text(provider.title).font(.callout.weight(.medium))
                    if let plan = account?.planLabel {
                        Text(plan)
                            .font(.caption2.weight(.semibold))
                            .padding(.horizontal, 5)
                            .padding(.vertical, 1)
                            .background(Color.accentColor.opacity(0.15), in: Capsule())
                    }
                }
                Text(detailText)
                    .font(.caption)
                    .foregroundStyle(detailIsWarning ? Color.orange : Color.secondary)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }

            Spacer()

            if model.loginInProgress == provider {
                ProgressView().controlSize(.small)
                Button("Cancel") { model.cancelLogin() }
                    .controlSize(.small)
            } else if account != nil {
                Menu {
                    Button("Switch Account…") { Task { await model.login(provider) } }
                    Button("Sign Out", role: .destructive) { Task { await model.logout(provider) } }
                } label: {
                    Image(systemName: "ellipsis")
                }
                .menuStyle(.borderlessButton)
                .menuIndicator(.hidden)
                .fixedSize()
                .disabled(model.loginInProgress != nil)
            } else {
                Button("Sign in") { Task { await model.login(provider) } }
                    .controlSize(.small)
                    .disabled(model.engineState != .running || model.loginInProgress != nil)
            }
        }
    }

    private var account: Credential? { model.accounts[provider] }

    private var detailText: String {
        if model.loginInProgress == provider { return "Finish signing in in your browser…" }
        guard let account else { return provider.subtitle }
        if account.disabled == true { return "Disabled" }
        if account.unavailable == true {
            return account.statusMessage.flatMap { $0.isEmpty ? nil : $0 } ?? "Cooling down (rate limited)"
        }
        return account.displayName
    }

    private var detailIsWarning: Bool {
        guard let account, model.loginInProgress != provider else { return false }
        return !account.isHealthy
    }
}

// MARK: - Connect

private struct ConnectPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var revealKey = false
    @State private var copied: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            CopyRow(label: "Base URL", value: model.baseURL, display: model.baseURL, copied: $copied)
            CopyRow(
                label: "API key",
                value: model.apiKey,
                display: revealKey ? model.apiKey : masked(model.apiKey),
                copied: $copied
            )
            .onTapGesture(count: 2) { revealKey.toggle() }

            Menu {
                ForEach(ClientSnippet.allCases) { snippet in
                    Button(snippet.title) {
                        model.copy(snippet.text(baseURL: model.baseURL, apiKey: model.apiKey))
                        flash(snippet.title)
                    }
                }
            } label: {
                Label(copied.map { "Copied \($0)" } ?? "Copy setup for…", systemImage: "doc.on.clipboard")
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
            .font(.caption)
            .padding(.top, 2)
        }
        .disabled(model.apiKey.isEmpty)
    }

    private func masked(_ key: String) -> String {
        guard key.count > 10 else { return String(repeating: "•", count: key.count) }
        return String(key.prefix(3)) + String(repeating: "•", count: 8) + String(key.suffix(4))
    }

    private func flash(_ what: String) {
        copied = what
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
            if copied == what { copied = nil }
        }
    }
}

private struct CopyRow: View {
    @EnvironmentObject private var model: AppModel
    let label: String
    let value: String
    let display: String
    @Binding var copied: String?

    var body: some View {
        HStack {
            Text(label).font(.caption).foregroundStyle(.secondary).frame(width: 58, alignment: .leading)
            Text(display)
                .font(.system(.caption, design: .monospaced))
                .lineLimit(1)
                .truncationMode(.middle)
            Spacer()
            Button {
                model.copy(value)
                copied = label
                DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
                    if copied == label { copied = nil }
                }
            } label: {
                Image(systemName: copied == label ? "checkmark" : "doc.on.doc")
            }
            .buttonStyle(.borderless)
            .help("Copy \(label)")
        }
    }
}
