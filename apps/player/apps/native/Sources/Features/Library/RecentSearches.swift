import SwiftUI

private struct RecentSearches: ViewModifier {
    @Binding var query: String
    @Environment(AppSession.self) private var session
    @State private var recents: [String] = []
    private var key: String { "kinosail.recentSearches.\(session.profileKey ?? "")" }

    func body(content: Content) -> some View {
        content
            .searchSuggestions {
                if query.isEmpty, !recents.isEmpty {
                    ForEach(recents, id: \.self) { recent in
                        Label(recent, systemImage: "clock.arrow.circlepath").searchCompletion(recent)
                            .accessibilityHint("Recent search")
                    }
                    Button("Clear recent searches", role: .destructive) { store([]) }
                }
            }
            .onSubmit(of: .search) { remember() }
            .onDisappear { remember() }
            .onAppear { recents = UserDefaults.standard.stringArray(forKey: key) ?? [] }
    }

    private func remember() {
        let text = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard (2...100).contains(text.count) else { return }
        store(Array(([text] + recents.filter { $0.caseInsensitiveCompare(text) != .orderedSame }).prefix(8)))
    }

    private func store(_ values: [String]) {
        recents = values
        UserDefaults.standard.set(values, forKey: key)
    }
}

extension View {
    func recentSearches(_ query: Binding<String>) -> some View {
        modifier(RecentSearches(query: query))
    }
}
