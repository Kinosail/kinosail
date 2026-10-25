import Observation
import SwiftUI

@MainActor @Observable
final class VideoPresenter {
    struct Request {
        let itemID: String
        let preview: MediaItem?
        let sourceID: UUID?
        let namespace: Namespace.ID?
    }

    fileprivate(set) var request: Request?

    func finish() { request = nil }

    func preview(for itemID: String?) -> MediaItem? {
        guard let request, request.itemID == itemID else { return nil }
        return request.preview
    }
}

extension AppSession {
    func presentVideo(_ itemID: String, preview: MediaItem? = nil, sourceID: UUID? = nil, namespace: Namespace.ID? = nil) {
        video.request = .init(itemID: itemID, preview: preview?.id == itemID ? preview : nil, sourceID: sourceID, namespace: namespace)
        showsVideoPlayer = true
    }

    var presentedVideoID: String? { video.request?.itemID ?? player.currentItem?.id }
}

private struct VideoZoomSourceKey: EnvironmentKey {
    static let defaultValue: UUID? = nil
}

extension EnvironmentValues {
    var videoZoomSourceID: UUID? {
        get { self[VideoZoomSourceKey.self] }
        set { self[VideoZoomSourceKey.self] = newValue }
    }
}

struct MediaDestinationLink<Label: View>: View {
    let destination: ScreenDestination
    var item: MediaItem?
    var zoomsFromLabel = true
    @ViewBuilder let label: () -> Label
    @Environment(AppSession.self) private var session
    @Environment(\.zoomNamespace) private var namespace
    @State private var sourceID = UUID()

    var body: some View {
        if case .playback(let id) = destination {
            Button { session.presentVideo(id, preview: item, sourceID: sourceID, namespace: namespace) } label: {
                label().environment(\.videoZoomSourceID, zoomsFromLabel ? nil : sourceID)
            }
            .videoZoomSource(zoomsFromLabel ? sourceID : nil, in: namespace)
        } else {
            NavigationLink(value: destination, label: label)
        }
    }
}

extension MediaDestinationLink where Label == Text {
    init(_ title: LocalizedStringKey, value: ScreenDestination, item: MediaItem? = nil) {
        self.init(destination: value, item: item) { Text(title) }
    }
}

extension AppSession {
    func open(_ destination: ScreenDestination, preview: MediaItem? = nil, otherwise: (ScreenDestination) -> Void) {
        if case .playback(let id) = destination { presentVideo(id, preview: preview) } else { otherwise(destination) }
    }
}

private struct VideoZoomSource: ViewModifier {
    let sourceID: UUID?
    let namespace: Namespace.ID?

    func body(content: Content) -> some View {
        #if os(iOS)
        if let namespace, let sourceID {
            content.matchedTransitionSource(id: sourceID, in: namespace) { $0.clipShape(.rect(cornerRadius: 12)) }
        } else { content }
        #else
        content
        #endif
    }
}

private struct VideoZoomArtwork: ViewModifier {
    @Environment(\.videoZoomSourceID) private var sourceID
    @Environment(\.zoomNamespace) private var namespace

    func body(content: Content) -> some View { content.videoZoomSource(sourceID, in: namespace) }
}

private struct VideoZoomDestination: ViewModifier {
    let request: VideoPresenter.Request?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    func body(content: Content) -> some View {
        #if os(iOS)
        if let sourceID = request?.sourceID, let namespace = request?.namespace, !reduceMotion {
            content.navigationTransition(.zoom(sourceID: sourceID, in: namespace))
        } else { content }
        #else
        content
        #endif
    }
}

extension View {
    func videoZoomSource(_ sourceID: UUID?, in namespace: Namespace.ID?) -> some View {
        modifier(VideoZoomSource(sourceID: sourceID, namespace: namespace))
    }

    func videoZoomArtwork() -> some View { modifier(VideoZoomArtwork()) }

    func videoZoomDestination(_ request: VideoPresenter.Request?) -> some View {
        modifier(VideoZoomDestination(request: request))
    }
}
