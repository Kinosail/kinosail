#if os(iOS)
import AppIntents
import Foundation

@MainActor enum IntentLibrary {
    static func open(action: MediaLink.Action, value: String, resume: Bool = false, session: AppSession) async throws {
        // Validate the request before reading credentials or making a request.
        let value = try Input.text(value, max: 512, label: "title or search", empty: resume)
        await session.restore()
        guard let client = session.client else { throw ClientError.invalidInput("Connect to your Server in Kinosail first.") }
        _ = try await client.viewer()
        let scope = try await client.profileScope()
        let result: MediaLink
        if action == .search { result = try MediaLink(action: action, value: value, scope: scope) }
        else {
            let page = try await client.library(query: value, view: resume ? .history : .all, limit: 200)
            let matches = page.items.filter { [.video, .music, .audiobook].contains($0.kind) &&
                (resume || $0.title.compare(value, options: [.caseInsensitive, .diacriticInsensitive]) == .orderedSame) }
            guard let item = matches.first else { throw ClientError.invalidInput("No playable title was found. Try searching in Kinosail.") }
            guard resume || matches.count == 1 && page.total <= page.items.count else {
                throw ClientError.invalidInput("More than one title matches. Choose it in Kinosail Search.")
            }
            result = try MediaLink(action: .play, value: item.id, scope: scope)
        }
        guard session.client?.identity == client.identity else { throw ClientError.invalidInput("Your connection changed. Try again.") }
        try session.openMediaLink(result)
    }
}

struct SearchLibraryIntent: AppIntent {
    static let title: LocalizedStringResource = "Search library"
    static let description = IntentDescription("Search your connected Kinosail library.")
    static let authenticationPolicy: IntentAuthenticationPolicy = .requiresAuthentication
    static let supportedModes: IntentModes = .foreground
    @Dependency private var session: AppSession
    @Parameter(title: "Search") var query: String
    static var parameterSummary: some ParameterSummary { Summary("Search for \(\.$query)") }
    @MainActor func perform() async throws -> some IntentResult {
        try await IntentLibrary.open(action: .search, value: query, session: session)
        return .result()
    }
}

struct PlayTitleIntent: AppIntent {
    static let title: LocalizedStringResource = "Play title"
    static let description = IntentDescription("Play a movie, episode, song, or audiobook by its exact title.")
    static let authenticationPolicy: IntentAuthenticationPolicy = .requiresAuthentication
    static let supportedModes: IntentModes = .foreground
    @Dependency private var session: AppSession
    @Parameter(title: "Title") var title: String
    static var parameterSummary: some ParameterSummary { Summary("Play \(\.$title)") }
    @MainActor func perform() async throws -> some IntentResult {
        try await IntentLibrary.open(action: .play, value: title, session: session)
        return .result()
    }
}

struct ResumeLibraryIntent: AppIntent {
    static let title: LocalizedStringResource = "Continue watching"
    static let description = IntentDescription("Resume the first playable title in your Kinosail history.")
    static let authenticationPolicy: IntentAuthenticationPolicy = .requiresAuthentication
    static let supportedModes: IntentModes = .foreground
    @Dependency private var session: AppSession
    @MainActor func perform() async throws -> some IntentResult {
        try await IntentLibrary.open(action: .play, value: "", resume: true, session: session)
        return .result()
    }
}

struct LibraryShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(intent: SearchLibraryIntent(), phrases: ["Search my library in \(.applicationName)"], shortTitle: "Search library", systemImageName: "magnifyingglass")
        AppShortcut(intent: PlayTitleIntent(), phrases: ["Play a title in \(.applicationName)"], shortTitle: "Play title", systemImageName: "play.fill")
        AppShortcut(intent: ResumeLibraryIntent(), phrases: ["Continue watching in \(.applicationName)"], shortTitle: "Continue watching", systemImageName: "play.rectangle")
    }
}
#endif
