import Foundation

struct LibraryEmptyState {
    let title: String
    let symbol: String
    let message: String

    init(view: LibraryView, hasQuery: Bool) {
        if hasQuery {
            (title, symbol, message) = ("No matches", "magnifyingglass", "Try another search or change the category.")
        } else {
            switch view {
            case .list: (title, symbol, message) = ("My List is empty", "bookmark", "Save a title with My List to keep it here.")
            case .history: (title, symbol, message) = ("Nothing to continue", "clock.arrow.circlepath", "Play or read something to pick up where you left off.")
            default: (title, symbol, message) = ("Nothing here yet", "play.rectangle", "Try another part of your library.")
            }
        }
    }
}
