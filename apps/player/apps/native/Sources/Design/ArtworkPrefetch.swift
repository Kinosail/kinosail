import SwiftUI

private struct ArtworkPrefetch: ViewModifier {
    let items: [MediaItem]
    let landscape: Bool
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase
    @Environment(\.dynamicTypeSize) private var dynamicType
    private struct Key: Hashable {
        let clientID: UUID?
        let paths: [String]
        let dimension: Int
        let active: Bool
    }
    func body(content: Content) -> some View {
        let key = Key(clientID: session.client?.identity,
                      paths: items.prefix(24).map { landscape ? $0.landscapeArtwork : $0.poster }.filter { !$0.isEmpty },
                      dimension: landscape || dynamicType.isAccessibilitySize ? 1600 : 800,
                      active: scenePhase == .active)
        content.task(id: key) {
            guard key.active, let client = session.client else { return }
            try? await session.artwork.prefetch(paths: key.paths, client: client, dimension: key.dimension)
        }
    }
}

extension View {
    func prefetchArtwork(_ items: [MediaItem], landscape: Bool = false) -> some View {
        modifier(ArtworkPrefetch(items: items, landscape: landscape))
    }
}
