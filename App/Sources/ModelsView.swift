import SwiftUI

/// Choose which models each subscription exposes, and give them extra names.
struct ModelsView: View {
    @EnvironmentObject private var model: AppModel
    @State private var provider: Provider = .codex
    @State private var search = ""

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Picker("Provider", selection: $provider) {
                    ForEach(Provider.allCases) { Text($0.title).tag($0) }
                }
                .pickerStyle(.segmented)
                .labelsHidden()
                .frame(width: 220)

                Spacer()

                TextField("Filter", text: $search)
                    .textFieldStyle(.roundedBorder)
                    .frame(width: 180)
            }
            .padding(12)

            Divider()

            if models.isEmpty {
                VStack(spacing: 6) {
                    Image(systemName: "square.stack.3d.up.slash").font(.title2)
                    Text(model.engineState == .running ? "No models in the catalog." : "Start the proxy to load models.")
                }
                .foregroundStyle(.secondary)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                List {
                    ForEach(models) { def in
                        ModelRow(provider: provider, definition: def)
                    }
                }
                .listStyle(.inset(alternatesRowBackgrounds: true))
            }

            Divider()

            HStack {
                Text("\(exposedCount) of \(allModels.count) exposed")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                if model.accounts[provider] == nil {
                    Text("· not signed in to \(provider.title)")
                        .font(.caption)
                        .foregroundStyle(.orange)
                }
                Spacer()
                Button("Hide All") { Task { await model.setAllExposed(false, for: provider) } }
                Button("Show All") { Task { await model.setAllExposed(true, for: provider) } }
            }
            .controlSize(.small)
            .padding(10)
        }
        .frame(minWidth: 560, minHeight: 420)
        .task { await model.refreshModels() }
    }

    private var allModels: [ModelDefinition] { model.catalog[provider] ?? [] }

    private var models: [ModelDefinition] {
        let q = search.trimmingCharacters(in: .whitespaces).lowercased()
        guard !q.isEmpty else { return allModels }
        return allModels.filter {
            $0.id.lowercased().contains(q) || ($0.displayName?.lowercased().contains(q) ?? false)
        }
    }

    private var exposedCount: Int {
        allModels.filter { model.isExposed($0.id, for: provider) }.count
    }
}

private struct ModelRow: View {
    @EnvironmentObject private var model: AppModel
    let provider: Provider
    let definition: ModelDefinition
    @State private var aliasDraft = ""

    var body: some View {
        HStack(spacing: 12) {
            Toggle("", isOn: Binding(
                get: { model.isExposed(definition.id, for: provider) },
                set: { on in Task { await model.setExposed(on, model: definition.id, for: provider) } }
            ))
            .toggleStyle(.checkbox)
            .labelsHidden()
            .help("Expose this model to clients")

            VStack(alignment: .leading, spacing: 2) {
                Text(definition.id).font(.system(.body, design: .monospaced))
                HStack(spacing: 6) {
                    if let name = definition.displayName { Text(name) }
                    if let ctx = definition.contextLength, ctx > 0 {
                        Text("· \(ctx / 1000)k context")
                    }
                }
                .font(.caption)
                .foregroundStyle(.secondary)
            }

            Spacer()

            TextField("alias", text: $aliasDraft)
                .textFieldStyle(.roundedBorder)
                .font(.system(.caption, design: .monospaced))
                .frame(width: 150)
                .help("Extra client-visible name for this model. The original ID keeps working.")
                .onSubmit(commitAlias)
        }
        .padding(.vertical, 3)
        .opacity(model.isExposed(definition.id, for: provider) ? 1 : 0.55)
        .onAppear { aliasDraft = model.alias(for: definition.id, provider: provider) }
        .onChange(of: provider) { _ in aliasDraft = model.alias(for: definition.id, provider: provider) }
    }

    private func commitAlias() {
        let current = model.alias(for: definition.id, provider: provider)
        guard aliasDraft != current else { return }
        Task { await model.setAlias(aliasDraft, model: definition.id, for: provider) }
    }
}
