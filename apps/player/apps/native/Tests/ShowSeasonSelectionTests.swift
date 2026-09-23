import Testing
@testable import KinosailPlayer

struct ShowSeasonSelectionTests {
    @Test func keepsASelectionThatStillExists() {
        #expect(ShowSeasonSelection.resolve(2, among: [1, 2, 3]) == 2)
    }

    @Test func fallsBackWhenTheSelectionIsUnavailable() {
        #expect(ShowSeasonSelection.resolve(4, among: [1, 2, 3]) == 1)
        #expect(ShowSeasonSelection.resolve(nil, among: [1, 2, 3]) == 1)
    }

    @Test func doesNotInventASeasonForAnEmptyShow() {
        #expect(ShowSeasonSelection.resolve(2, among: []) == nil)
    }
}
