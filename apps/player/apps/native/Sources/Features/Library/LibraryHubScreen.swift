import SwiftUI

struct LibraryHubScreen: View {
    var mode: PlayerMode?
    #if os(tvOS)
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    #endif
    var body: some View {
        #if os(tvOS)
        ScrollView {
            VStack(alignment: .leading, spacing: 36) {
                Text("Library").font(.largeTitle.bold()).accessibilityAddTraits(.isHeader)
                if mode != .listen { hubSection("Watch", [
                    ("Movies", "film", .library(.movies)), ("Shows", "tv", .library(.shows))
                ]) }
                if mode != .watch { hubSection("Listen", [
                    ("Music", "music.note", .library(.music)), ("Audiobooks", "headphones", .library(.audiobooks))
                ]) }
                if mode == nil { hubSection("Explore", [
                    ("Photos", "photo.on.rectangle", .library(.photos)), ("All media", "square.grid.2x2", .library(.all))
                ]) }
                hubSection("Your library", [
                    ("My List", "star", .library(.list)), ("Collections", "rectangle.stack", .collections),
                    ("History", "clock", .library(.history))
                ])
            }
            .padding(.horizontal, KinoTheme.contentPadding)
            .padding(.vertical, 32)
        }
        .scrollClipDisabled()
        .cinemaBackground()
        .navigationTitle("")
        #else
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
        #endif
    }

    #if os(tvOS)
    private func hubSection(_ title: String, _ links: [(String, String, ScreenDestination)]) -> some View {
        VStack(alignment: .leading, spacing: 18) {
            Text(title).font(.title2.bold()).accessibilityAddTraits(.isHeader)
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 24),
                                     count: dynamicTypeSize.isAccessibilitySize ? 1 : 3), spacing: 24) {
                ForEach(links.indices, id: \.self) { index in
                    NavigationLink(value: links[index].2) {
                        HStack(spacing: 24) {
                            Image(systemName: links[index].1).font(.title).frame(width: 72)
                                .foregroundStyle(KinoTheme.signal)
                            Text(links[index].0).font(.title3.weight(.semibold))
                            Spacer(minLength: 0)
                        }
                        .padding(28)
                        .frame(maxWidth: .infinity, minHeight: 112, alignment: .leading)
                        .background(KinoTheme.surface, in: RoundedRectangle(cornerRadius: 18))
                    }
                    .buttonStyle(.card)
                }
            }
            .focusSection()
        }
    }
    #endif
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
            .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
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
