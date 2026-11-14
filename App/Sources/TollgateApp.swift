import SwiftUI

@main
struct TollgateApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var model = AppModel.shared
    @StateObject private var updater = Updater.shared

    var body: some Scene {
        MenuBarExtra {
            MenuContentView()
                .environmentObject(model)
                .environmentObject(updater)
        } label: {
            Image(systemName: model.menuBarSymbol)
        }
        .menuBarExtraStyle(.window)

        Window("Models", id: WindowID.models) {
            ModelsView()
                .environmentObject(model)
        }
        .defaultSize(width: 620, height: 520)

        Window("CLI Tollgate Settings", id: WindowID.settings) {
            SettingsView()
                .environmentObject(model)
                .environmentObject(updater)
        }
        .defaultSize(width: 460, height: 620)
        .windowResizability(.contentMinSize)
    }
}

enum WindowID {
    static let models = "models"
    static let settings = "settings"
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        AppModel.shared.start()
    }

    func applicationWillTerminate(_ notification: Notification) {
        AppModel.shared.engine.stopSynchronously()
    }
}
