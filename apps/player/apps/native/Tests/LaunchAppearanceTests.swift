import Testing
import UIKit
@testable import KinosailPlayer

struct LaunchAppearanceTests {
    @Test func systemLaunchMatchesAnimatedCover() throws {
        let launch = try #require(Bundle.main.infoDictionary?["UILaunchScreen"] as? [String: String])
        #expect(launch["UIImageName"] == "LaunchMark")
        #expect(launch["UIColorName"] == "LaunchBlack")
        #expect(UIImage(named: "LaunchMark") != nil)
        let color = try #require(UIColor(named: "LaunchBlack"))
        var white: CGFloat = 1
        var alpha: CGFloat = 0
        #expect(color.getWhite(&white, alpha: &alpha))
        #expect(white == 0 && alpha == 1)
    }
}
