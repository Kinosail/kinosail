import Foundation
import Testing
@testable import KinosailPlayer

struct InterfaceLocalizationTests {
    @Test(arguments: [("es", "Inicio", "Series"), ("fr", "Accueil", "Émissions"),
                      ("ar", "الصفحة الرئيسية", "العروض")])
    func navigationUsesSharedTranslations(_ language: String, _ home: String, _ shows: String) {
        let path = Bundle.main.path(forResource: language, ofType: "lproj")
        #expect(path != nil)
        guard let path, let bundle = Bundle(path: path) else { return }
        #expect(bundle.localizedString(forKey: "Home", value: nil, table: nil) == home)
        #expect(bundle.localizedString(forKey: "Shows", value: nil, table: nil) == shows)
    }

    @Test func navigationUsesTheSameEnglishTermsAsWeb() {
        #expect(PlayerTab.home.title == "Home")
        #expect(PlayerTab.shows.title == "Shows")
    }
}
