import Testing
@testable import KinosailPlayer

struct LibraryEmptyStateTests {
    @Test func guidesPeopleFromAnEmptyList() {
        let state = LibraryEmptyState(view: .list, hasQuery: false)
        #expect(state.title == "My List is empty")
        #expect(state.message.contains("My List"))
    }

    @Test func explainsAnEmptyHistory() {
        let state = LibraryEmptyState(view: .history, hasQuery: false)
        #expect(state.title == "Nothing to continue")
        #expect(state.message.contains("pick up where you left off"))
    }

    @Test func searchResultTakesPriorityOverTheSection() {
        let state = LibraryEmptyState(view: .list, hasQuery: true)
        #expect(state.title == "No matches")
        #expect(state.symbol == "magnifyingglass")
    }

    @Test func otherEmptySectionsRemainNeutral() {
        let state = LibraryEmptyState(view: .movies, hasQuery: false)
        #expect(state.title == "Nothing here yet")
    }
}
