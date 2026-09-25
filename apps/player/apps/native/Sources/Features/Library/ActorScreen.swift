import SwiftUI

struct ActorScreen: View {
    let name: String
    @Environment(AppSession.self) private var session
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(name)", refreshID: session.contentRevision.uuidString, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.actor(name: name, policy: policy)
            }) { actor in
                VStack(alignment: .leading, spacing: 28) {
                    #if os(tvOS)
                    HStack(alignment: .bottom, spacing: 32) {
                        if !actor.image.isEmpty {
                            Artwork(path: actor.image, symbol: "person.fill", dimension: 800)
                                .frame(width: 200).clipShape(.rect(cornerRadius: 12))
                        }
                        VStack(alignment: .leading, spacing: 8) {
                            Text(name).font(.system(.largeTitle, design: .rounded).bold())
                                .accessibilityAddTraits(.isHeader)
                            let count = actor.movies.count + actor.shows.count
                            Text("\(count) \(count == 1 ? "title" : "titles") in your library")
                                .foregroundStyle(KinoTheme.muted)
                        }
                    }
                    #else
                    if !actor.image.isEmpty {
                        Artwork(path: actor.image, symbol: "person.fill", dimension: 800)
                            .frame(maxWidth: 200).clipShape(.rect(cornerRadius: 12))
                    }
                    Text("In your library").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                    #endif
                    credits("Movies", items: actor.movies, shows: false)
                    credits("TV Shows", items: actor.shows, shows: true)
                    if actor.movies.isEmpty && actor.shows.isEmpty {
                        ContentUnavailableView("No titles available", systemImage: "film",
                                               description: Text("No matching titles are available in your library."))
                    }
                }.frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(KinoTheme.contentPadding)
        }
        #if os(tvOS)
        .cinemaBackground()
        .navigationTitle("")
        #else
        .background(KinoTheme.background)
        .navigationTitle(name)
        .navigationBarTitleDisplayMode(.inline)
        #endif
    }

    @ViewBuilder private func credits(_ title: String, items: [ActorDetail.Credit], shows: Bool) -> some View {
        if !items.isEmpty {
            VStack(alignment: .leading, spacing: 16) {
                Text(title).font(.title3.bold()).accessibilityAddTraits(.isHeader)
                LazyVGrid(columns: MediaGrid.columns(landscape: false, accessibility: dynamicTypeSize.isAccessibilitySize), alignment: .leading, spacing: 28) {
                    ForEach(items) { item in
                        NavigationLink(value: shows ? ScreenDestination.show(item.id) : .detail(item.id)) {
                            VStack(alignment: .leading, spacing: 8) {
                                Artwork(path: item.artwork, symbol: shows ? "tv" : "film", dimension: 800)
                                    .clipShape(.rect(cornerRadius: 12))
                                Text(item.title).font(.headline).foregroundStyle(KinoTheme.text)
                                    .lineLimit(dynamicTypeSize.isAccessibilitySize ? nil : 2)
                                if !item.year.isEmpty { Text(item.year).font(.caption).foregroundStyle(KinoTheme.muted) }
                                if !item.role.isEmpty { Text(item.role).font(.caption).foregroundStyle(KinoTheme.muted)
                                    .lineLimit(dynamicTypeSize.isAccessibilitySize ? nil : 2) }
                            }
                            .fixedSize(horizontal: false, vertical: true)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .contentShape(.rect)
                        }
                        #if os(iOS)
                        .buttonStyle(.plain)
                        #else
                        .buttonStyle(.card)
                        #endif
                        .accessibilityElement(children: .combine)
                    }
                }
                #if os(tvOS)
                .padding(.vertical, 24)
                .focusSection()
                #endif
            }
        }
    }
}
