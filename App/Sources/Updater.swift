import AppKit
import Combine
import Sparkle

/// Sparkle auto-updates. The feed URL and EdDSA public key live in Info.plist
/// (SUFeedURL / SUPublicEDKey), set from project.yml.
@MainActor
final class Updater: NSObject, ObservableObject, SPUStandardUserDriverDelegate {
    static let shared = Updater()

    @Published private(set) var canCheckForUpdates = false
    @Published var automaticallyChecks: Bool {
        didSet { controller.updater.automaticallyChecksForUpdates = automaticallyChecks }
    }
    @Published var automaticallyDownloads: Bool {
        didSet { controller.updater.automaticallyDownloadsUpdates = automaticallyDownloads }
    }

    private var controller: SPUStandardUpdaterController!
    private var cancellables: Set<AnyCancellable> = []

    private override init() {
        automaticallyChecks = true
        automaticallyDownloads = true
        super.init()
        controller = SPUStandardUpdaterController(startingUpdater: true, updaterDelegate: nil, userDriverDelegate: self)
        automaticallyChecks = controller.updater.automaticallyChecksForUpdates
        automaticallyDownloads = controller.updater.automaticallyDownloadsUpdates
        controller.updater.publisher(for: \.canCheckForUpdates)
            .receive(on: RunLoop.main)
            .sink { [weak self] in self?.canCheckForUpdates = $0 }
            .store(in: &cancellables)
    }

    func checkForUpdates() {
        NSApp.activate(ignoringOtherApps: true)
        controller.checkForUpdates(nil)
    }

    // A menu bar app has no Dock icon, so let Sparkle use gentle reminders
    // instead of stealing focus for scheduled update alerts.
    nonisolated var supportsGentleScheduledUpdateReminders: Bool { true }
}
