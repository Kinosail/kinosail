import SwiftUI

struct DeferredRetry: Equatable, Sendable {
    let id = UUID()
    let seconds: Int
    let message: String

    init?(_ error: any Error) {
        guard case ClientError.retryAfter(let seconds) = error else { return nil }
        self.seconds = seconds
        message = ClientError.retryAfter(seconds).localizedDescription
    }
}

struct DeferredLoadingState: View {
    let retry: DeferredRetry
    var layout = LoadingLayout.shelf

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text(retry.message).font(.callout).foregroundStyle(KinoTheme.muted)
                .fixedSize(horizontal: false, vertical: true)
            LoadingState(title: retry.message, layout: layout)
        }
    }
}

extension View {
    func retrying(after retry: DeferredRetry?, _ action: @escaping @MainActor () async -> Void) -> some View {
        task(id: retry) {
            guard let retry else { return }
            do { try await Task.sleep(for: .seconds(retry.seconds)) } catch { return }
            await action()
        }
    }
}
