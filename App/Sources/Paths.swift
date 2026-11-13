import Foundation
import Security

/// File locations owned by the app. Everything lives under
/// ~/Library/Application Support/CLI Tollgate so uninstalling is one folder.
enum Paths {
    static let appSupport: URL = {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        return base.appendingPathComponent("CLI Tollgate", isDirectory: true)
    }()

    static var config: URL { appSupport.appendingPathComponent("config.yaml") }
    static var auths: URL { appSupport.appendingPathComponent("auths", isDirectory: true) }
    static var logs: URL { appSupport.appendingPathComponent("logs", isDirectory: true) }
    static var engineLog: URL { logs.appendingPathComponent("engine.log") }
    static var pidFile: URL { appSupport.appendingPathComponent("engine.pid") }

    /// The proxy binary embedded in Contents/MacOS by the build.
    static var engineBinary: URL? {
        Bundle.main.url(forAuxiliaryExecutable: "tollgate-engine")
    }

    static func ensureDirectories() throws {
        let fm = FileManager.default
        for dir in [appSupport, auths, logs] {
            try fm.createDirectory(at: dir, withIntermediateDirectories: true,
                                   attributes: [.posixPermissions: 0o700])
        }
    }
}

/// Creates the first config.yaml and reads the few values the app needs
/// before the management API is reachable. After bootstrap, the engine owns
/// the file and every change goes through the management API.
enum EngineConfig {
    static let defaultPort = 8317

    static func bootstrapIfNeeded() throws {
        guard !FileManager.default.fileExists(atPath: Paths.config.path) else { return }
        let yaml = template(port: defaultPort, apiKey: generateAPIKey(), authDir: Paths.auths.path)
        try yaml.write(to: Paths.config, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: Paths.config.path)
    }

    static func template(port: Int, apiKey: String, authDir: String) -> String {
        """
        # Managed by CLI Tollgate. Edit through the app; manual edits are hot-reloaded.
        config-version: 8

        server:
          # Loopback only. Web pages can still call it (CORS is open, API key required).
          host: "127.0.0.1"
          port: \(port)

        management:
          allow-remote: false
          # The app passes a per-launch MANAGEMENT_PASSWORD; no key is stored here.
          secret-key: ""
          disable-control-panel: true

        access:
          api-keys:
            - "\(apiKey)"

        oauth:
          auth-dir: "\(authDir)"

        upstream:
          codex:
            disable-codex-cloaking: false
          claude:
            disable-claude-cloak-mode: false
            disable-cloaking-model-list: false

        usage-tracker:
          enabled: true

        """
    }

    static func generateAPIKey() -> String {
        var bytes = [UInt8](repeating: 0, count: 24)
        _ = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        return "tg-" + bytes.map { String(format: "%02x", $0) }.joined()
    }

    /// Reads `server.port` from config.yaml without a YAML dependency.
    /// The engine rewrites the file in normalized block style, so a scan of the
    /// top-level `server:` block is reliable enough.
    static func readPort() -> Int {
        guard let text = try? String(contentsOf: Paths.config, encoding: .utf8) else { return defaultPort }
        var inServer = false
        var childIndent: Int?
        for rawLine in text.components(separatedBy: .newlines) {
            let trimmed = rawLine.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty || trimmed.hasPrefix("#") { continue }
            let indent = rawLine.prefix { $0 == " " }.count
            if indent == 0 {
                inServer = (trimmed == "server:")
                childIndent = nil
                continue
            }
            guard inServer else { continue }
            if childIndent == nil { childIndent = indent }
            guard indent == childIndent, trimmed.hasPrefix("port:") else { continue }
            let value = trimmed.dropFirst("port:".count)
                .trimmingCharacters(in: CharacterSet(charactersIn: " \"'"))
            if let port = Int(value), (1...65535).contains(port) { return port }
        }
        return defaultPort
    }
}
