import AppKit
import ServiceManagement

let kinosailAddress = URL(string: "https://localhost:38127/")!

struct ServerFiles {
    let helpers: URL
    let support: URL
    let cache: URL
    let logs: URL

    static func current() -> ServerFiles {
        let manager = FileManager.default
        let library = manager.urls(for: .libraryDirectory, in: .userDomainMask)[0]
        let helpers = Bundle.main.bundleURL.appendingPathComponent("Contents/Helpers", isDirectory: true)
        return ServerFiles(helpers: helpers,
                           support: library.appendingPathComponent("Application Support/Kinosail", isDirectory: true),
                           cache: library.appendingPathComponent("Caches/Kinosail", isDirectory: true),
                           logs: library.appendingPathComponent("Logs/Kinosail", isDirectory: true))
    }

    var executable: URL { helpers.appendingPathComponent("kinosail") }
    var log: URL { logs.appendingPathComponent("kinosail.log") }

    func prepare() throws {
        for folder in [support.appendingPathComponent("data"), support.appendingPathComponent("backups"), cache, logs] {
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        }
        let attributes = try? FileManager.default.attributesOfItem(atPath: log.path)
        if let size = attributes?[.size] as? Int, size > 10 << 20 {
            let previous = logs.appendingPathComponent("kinosail.log.1")
            try? FileManager.default.removeItem(at: previous)
            try FileManager.default.moveItem(at: log, to: previous)
        }
        if !FileManager.default.fileExists(atPath: log.path) {
            FileManager.default.createFile(atPath: log.path, contents: nil, attributes: [.posixPermissions: 0o600])
        }
    }

    func environment() -> [String: String] {
        var values = ProcessInfo.processInfo.environment.filter { !$0.key.hasPrefix("KINOSAIL_") }
        values["KINOSAIL_DATA_DIR"] = support.appendingPathComponent("data").path
        values["KINOSAIL_BACKUP_DIR"] = support.appendingPathComponent("backups").path
        values["KINOSAIL_CACHE_DIR"] = cache.path
        values["KINOSAIL_MEDIA_DIR"] = "/"
        values["KINOSAIL_SUPERVISED"] = "true"
        values["KINOSAIL_STOP_WITH_LAUNCHER"] = "true"
        values["KINOSAIL_FFMPEG"] = helpers.appendingPathComponent("ffmpeg").path
        values["KINOSAIL_FFPROBE"] = helpers.appendingPathComponent("ffprobe").path
        values["KINOSAIL_FPCALC"] = helpers.appendingPathComponent("fpcalc").path
        return values
    }
}

final class Launcher: NSObject, NSApplicationDelegate, NSMenuDelegate {
    private let files = ServerFiles.current()
    private let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let status = NSMenuItem(title: "Starting Kinosail…", action: nil, keyEquivalent: "")
    private let login = NSMenuItem(title: "Start Kinosail when I log in", action: #selector(toggleLogin), keyEquivalent: "")
    private let restart = NSMenuItem(title: "Restart Kinosail", action: #selector(restartServer), keyEquivalent: "")
    private var server: Process?
    private var input: Pipe?
    private var quitting = false
    private var crashes: [Date] = []
    private var termination: DispatchSourceSignal?
    private let defaults = UserDefaults.standard

    func applicationDidFinishLaunching(_ notification: Notification) {
        let image = NSImage(systemSymbolName: "play.tv.fill", accessibilityDescription: "Kinosail")
        image?.isTemplate = true
        item.button?.image = image
        item.button?.toolTip = "Kinosail"
        let menu = NSMenu()
        menu.delegate = self
        menu.autoenablesItems = false
        status.isEnabled = false
        let open = NSMenuItem(title: "Open Kinosail", action: #selector(openKinosail), keyEquivalent: "o")
        let quit = NSMenuItem(title: "Quit Kinosail", action: #selector(quitKinosail), keyEquivalent: "q")
        for entry in [status, NSMenuItem.separator(), open, restart, login, NSMenuItem.separator(), quit] {
            if entry.action != nil { entry.target = self }
            menu.addItem(entry)
        }
        item.menu = menu
        watchTermination()
        if !defaults.bool(forKey: "loginItemConfigured") {
            try? SMAppService.mainApp.register()
            defaults.set(true, forKey: "loginItemConfigured")
        }
        startServer()
        waitUntilHealthy(open: !defaults.bool(forKey: "openedSetup"))
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        openKinosail()
        return false
    }

    func menuWillOpen(_ menu: NSMenu) {
        login.state = SMAppService.mainApp.status == .enabled ? .on : .off
    }

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        quitting = true
        stopServer {
            NSApp.reply(toApplicationShouldTerminate: true)
        }
        return .terminateLater
    }

    @objc func openKinosail() {
        NSWorkspace.shared.open(kinosailAddress)
    }

    @objc func quitKinosail() {
        NSApp.terminate(nil)
    }

    @objc func restartServer() {
        crashes.removeAll()
        if server?.isRunning == true {
            input?.fileHandleForWriting.closeFile()
        } else {
            startServer()
            waitUntilHealthy(open: false)
        }
    }

    @objc func toggleLogin() {
        if SMAppService.mainApp.status == .enabled {
            try? SMAppService.mainApp.unregister()
        } else {
            try? SMAppService.mainApp.register()
        }
    }

    private func watchTermination() {
        signal(SIGTERM, SIG_IGN)
        let source = DispatchSource.makeSignalSource(signal: SIGTERM, queue: .main)
        source.setEventHandler { NSApp.perform(#selector(NSApplication.terminate(_:)), with: nil, afterDelay: 0) }
        source.resume()
        termination = source
    }

    private func startServer() {
        do {
            try files.prepare()
            let process = Process()
            let pipe = Pipe()
            let log = try FileHandle(forWritingTo: files.log)
            log.seekToEndOfFile()
            process.executableURL = files.executable
            process.environment = files.environment()
            process.currentDirectoryURL = files.support
            process.standardInput = pipe
            process.standardOutput = log
            process.standardError = log
            process.terminationHandler = { [weak self] finished in
                DispatchQueue.main.async { self?.serverStopped(finished.terminationStatus) }
            }
            try process.run()
            server = process
            input = pipe
            status.title = "Starting Kinosail…"
            restart.isEnabled = true
        } catch {
            status.title = "Kinosail could not start"
        }
    }

    private func serverStopped(_ code: Int32) {
        server = nil
        input = nil
        if quitting { return }
        if code != 0 {
            let now = Date()
            crashes = crashes.filter { now.timeIntervalSince($0) < 120 } + [now]
            if crashes.count >= 5 {
                status.title = "Kinosail stopped. Choose Restart to try again."
                return
            }
        }
        status.title = "Restarting Kinosail…"
        DispatchQueue.main.asyncAfter(deadline: .now() + (code == 0 ? 0.5 : 3)) { [weak self] in
            guard let self, !self.quitting, self.server == nil else { return }
            self.startServer()
            self.waitUntilHealthy(open: false)
        }
    }

    private func stopServer(then finish: @escaping () -> Void) {
        guard let process = server, process.isRunning else { finish(); return }
        input?.fileHandleForWriting.closeFile()
        DispatchQueue.global().async {
            let deadline = Date().addingTimeInterval(20)
            while process.isRunning && Date() < deadline { Thread.sleep(forTimeInterval: 0.1) }
            if process.isRunning { process.terminate() }
            process.waitUntilExit()
            DispatchQueue.main.async(execute: finish)
        }
    }

    private func waitUntilHealthy(open: Bool) {
        let executable = files.executable
        let environment = files.environment()
        DispatchQueue.global().async { [weak self] in
            for _ in 0..<120 {
                let check = Process()
                check.executableURL = executable
                check.arguments = ["healthcheck"]
                check.environment = environment
                check.standardOutput = FileHandle.nullDevice
                check.standardError = FileHandle.nullDevice
                if (try? check.run()) != nil {
                    check.waitUntilExit()
                    if check.terminationStatus == 0 {
                        DispatchQueue.main.async { self?.serverHealthy(open: open) }
                        return
                    }
                }
                Thread.sleep(forTimeInterval: 0.5)
            }
            DispatchQueue.main.async {
                if self?.server?.isRunning == true { self?.status.title = "Kinosail is taking longer than usual to start" }
            }
        }
    }

    private func serverHealthy(open: Bool) {
        guard server?.isRunning == true else { return }
        status.title = "Kinosail is running"
        if open {
            defaults.set(true, forKey: "openedSetup")
            openKinosail()
        }
    }
}

let application = NSApplication.shared
let launcher = Launcher()
application.delegate = launcher
application.setActivationPolicy(.accessory)
application.run()
