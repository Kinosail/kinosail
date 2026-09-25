import SwiftUI

struct LibraryHubScreen: View {
    var mode: PlayerMode?
    var body: some View {
        List {
            if mode != .listen { Section("Watch") {
                LibraryDestinationLink("Movies", "film", .library(.movies))
                LibraryDestinationLink("Shows", "tv", .library(.shows))
            } }
            if mode != .watch { Section("Listen") {
                LibraryDestinationLink("Music", "music.note", .library(.music))
                LibraryDestinationLink("Audiobooks", "headphones", .library(.audiobooks))
            } }
            if mode == nil { Section("Explore") {
                #if os(iOS)
                LibraryDestinationLink("Ebooks & comics", "books.vertical", .library(.books))
                #endif
                LibraryDestinationLink("Photos", "photo.on.rectangle", .library(.photos))
                LibraryDestinationLink("All media", "square.grid.2x2", .library(.all))
            } }
            Section("Your library") {
                LibraryDestinationLink("My List", "star", .library(.list))
                LibraryDestinationLink("Collections", "rectangle.stack", .collections)
                LibraryDestinationLink("History", "clock", .library(.history))
            }
        }
        #if os(iOS)
        .scrollContentBackground(.hidden)
        #endif
        .background(KinoTheme.background)
        .navigationTitle("Library")
    }
}

struct LibraryQuickLinks: View {
    var mode: PlayerMode?
    var body: some View {
        ScrollView(.horizontal) {
            HStack(spacing: 12) {
                if mode != .listen {
                    LibraryDestinationLink("Movies", "film", .library(.movies))
                    LibraryDestinationLink("Shows", "tv", .library(.shows))
                }
                if mode != .watch {
                    LibraryDestinationLink("Music", "music.note", .library(.music))
                    if mode == .listen { LibraryDestinationLink("Audiobooks", "headphones", .library(.audiobooks)) }
                }
                if mode == nil { LibraryDestinationLink("All media", "square.grid.2x2", .library(.all)) }
            }
            .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.secondaryControlInk)
            .padding(.vertical, 8)
        }
        .scrollIndicators(.hidden)
        #if os(tvOS)
        .focusSection()
        #endif
    }
}

struct LibraryDestinationLink: View {
    let title: String
    let symbol: String
    let destination: ScreenDestination

    init(_ title: String, _ symbol: String, _ destination: ScreenDestination) {
        self.title = title
        self.symbol = symbol
        self.destination = destination
    }

    var body: some View {
        NavigationLink(value: destination) {
            Label(title, systemImage: symbol)
                .frame(minHeight: 44, alignment: .leading)
                .contentShape(.rect)
        }
    }
}
