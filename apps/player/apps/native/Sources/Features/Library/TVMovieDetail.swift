import Foundation

#if os(tvOS)
struct TVMovieDetail {
    let item: MediaItem
    var usesBackdrop: Bool { item.kind == .video && !item.backdrop.isEmpty }
    var title: String { item.show.isEmpty ? item.title : ShowSeasonSelection.episodeTitle(item) }
    var metadata: String {
        let facts = item.show.isEmpty ? [item.year, item.rating, item.genres]
            : ["Season \(item.season), Episode \(item.episode)", item.year, item.rating]
        return facts.filter { !$0.isEmpty }.joined(separator: " · ")
    }
}
#endif
