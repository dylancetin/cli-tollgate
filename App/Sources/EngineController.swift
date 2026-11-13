import Darwin
import Foundation
import Security

/// Runs the bundled CLIProxyAPI engine as a child process bound to 127.0.0.1.
@MainActor
final class EngineController {
    enum State: Equatable {
        case stopped
        case starting
        case running
        case failed(String)
    }

    /// Random per launch, handed to the engine through MANAGEMENT_PASSWORD.
    let managementPassword: String = {
        var bytes = [UInt8](repeating: 0, count: 32)
        _ = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        return bytes.map { String(format: "%02x", $0) }.joined()
    }()

    private(set) var state: State = .stopped {
        didSet { if state != oldValue { onStateChange?(state) } }
    }
    private(set) var port: Int = EngineConfig.defaultPort
    var onStateChange: ((State) -> Void)?

    private var process: Process?
    private var intentionalStop = false
    private var recentCrashes: [Date] = []
    private var healthTask: Task<Void, Never>?

    var client: ManagementClient {
        ManagementClient(port: port, password: managementPassword)
    }

    func start() {
        guard process == nil else { return }
        intentionalStop = false
        state = .starting
        do {
            try Paths.ensureDirectories()
            try EngineConfig.bootstrapIfNeeded()
            port = EngineConfig.readPort()
            killStaleEngine()
            try launch()
        } catch {
            state = .failed(error.localizedDescription)
        }
    }

    func stop() {
        intentionalStop = true
        healthTask?.cancel()
        process?.terminate()
        state = .stopped
    }

    func restart() {
        stop()
        // Wait for the old process to release the port before relaunching.
        let old = process
        Task { @MainActor in
            if let old {
                for _ in 0..<50 where old.isRunning {
                    try? await Task.sleep(nanoseconds: 100_000_000)
                }
                if old.isRunning { kill(old.processIdentifier, SIGKILL) }
            }
            self.process = nil
            self.recentCrashes.removeAll()
            self.start()
        }
    }

    /// Called from applicationWillTerminate: the run loop will not spin again.
    func stopSynchronously() {
        intentionalStop = true
        healthTask?.cancel()
        guard let process, process.isRunning else { return }
        process.terminate()
        let deadline = Date().addingTimeInterval(3)
        while process.isRunning && Date() < deadline { usleep(50_000) }
        if process.isRunning { kill(process.processIdentifier, SIGKILL) }
        try? FileManager.default.removeItem(at: Paths.pidFile)
    }

    // MARK: - Process

    private func launch() throws {
        guard let binary = Paths.engineBinary else {
            throw EngineError("The proxy engine is missing from the app bundle.")
        }
        rotateLogIfNeeded()
        FileManager.default.createFile(atPath: Paths.engineLog.path, contents: nil)
        let log = try FileHandle(forWritingTo: Paths.engineLog)
        log.seekToEndOfFile()

        let p = Process()
        p.executableURL = binary
        p.arguments = ["--config", Paths.config.path]
        p.currentDirectoryURL = Paths.appSupport
        var env = ProcessInfo.processInfo.environment
        env["MANAGEMENT_PASSWORD"] = managementPassword
        p.environment = env
        p.standardOutput = log
        p.standardError = log
        p.standardInput = FileHandle.nullDevice
        p.terminationHandler = { [weak self] proc in
            let status = proc.terminationStatus
            Task { @MainActor in self?.handleExit(of: proc, status: status) }
        }
        try p.run()
        process = p
        try? String(p.processIdentifier).write(to: Paths.pidFile, atomically: true, encoding: .utf8)
        waitUntilHealthy()
    }

    private func waitUntilHealthy() {
        healthTask?.cancel()
        let client = self.client
        healthTask = Task { @MainActor [weak self] in
            for _ in 0..<60 {
                if Task.isCancelled { return }
                if await client.isHealthy() {
                    if self?.state == .starting { self?.state = .running }
                    return
                }
                try? await Task.sleep(nanoseconds: 250_000_000)
            }
            if self?.state == .starting {
                self?.state = .failed("The engine did not answer on port \(self?.port ?? 0). \(Self.lastLogLine())")
            }
        }
    }

    private func handleExit(of proc: Process, status: Int32) {
        guard proc === process else { return }
        process = nil
        healthTask?.cancel()
        try? FileManager.default.removeItem(at: Paths.pidFile)
        if intentionalStop {
            state = .stopped
            return
        }
        // Restart on crash, but give up after 3 crashes within a minute.
        let now = Date()
        recentCrashes = recentCrashes.filter { now.timeIntervalSince($0) < 60 } + [now]
        if recentCrashes.count >= 3 {
            state = .failed("The engine exited (status \(status)). \(Self.lastLogLine())")
            return
        }
        state = .starting
        Task { @MainActor in
            try? await Task.sleep(nanoseconds: 1_000_000_000)
            guard self.process == nil, !self.intentionalStop else { return }
            do { try self.launch() } catch { self.state = .failed(error.localizedDescription) }
        }
    }

    /// Kills an engine left behind by a previous crash of this app, but only
    /// when the pid still points at our own bundled binary.
    private func killStaleEngine() {
        guard let text = try? String(contentsOf: Paths.pidFile, encoding: .utf8),
              let pid = pid_t(text.trimmingCharacters(in: .whitespacesAndNewlines)),
              pid > 0, kill(pid, 0) == 0,
              let ours = Paths.engineBinary?.resolvingSymlinksInPath().path else { return }
        var buffer = [CChar](repeating: 0, count: 4096)
        guard proc_pidpath(pid, &buffer, UInt32(buffer.count)) > 0 else { return }
        let path = String(cString: buffer)
        if URL(fileURLWithPath: path).resolvingSymlinksInPath().path == ours
            || path.hasSuffix("/tollgate-engine") {
            kill(pid, SIGTERM)
            usleep(500_000)
            if kill(pid, 0) == 0 { kill(pid, SIGKILL) }
        }
        try? FileManager.default.removeItem(at: Paths.pidFile)
    }

    private func rotateLogIfNeeded() {
        let fm = FileManager.default
        guard let size = (try? fm.attributesOfItem(atPath: Paths.engineLog.path))?[.size] as? Int,
              size > 5_000_000 else { return }
        let old = Paths.logs.appendingPathComponent("engine.old.log")
        try? fm.removeItem(at: old)
        try? fm.moveItem(at: Paths.engineLog, to: old)
    }

    static func lastLogLine() -> String {
        guard let handle = try? FileHandle(forReadingFrom: Paths.engineLog) else { return "" }
        defer { try? handle.close() }
        let size = handle.seekToEndOfFile()
        handle.seek(toFileOffset: size > 4096 ? size - 4096 : 0)
        let text = String(decoding: handle.readDataToEndOfFile(), as: UTF8.self)
        return text.split(whereSeparator: \.isNewline).last.map(String.init) ?? ""
    }
}

struct EngineError: LocalizedError {
    let message: String
    init(_ message: String) { self.message = message }
    var errorDescription: String? { message }
}
