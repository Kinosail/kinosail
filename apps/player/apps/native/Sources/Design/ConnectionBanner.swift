import SwiftUI

struct ConnectionBanner: View {
    let openDownloads: () -> Void
    @Environment(AppSession.self) private var session

    var body: some View {
        if session.connection.unavailable {
            ViewThatFits(in: .horizontal) {
                HStack(spacing: 20) { message; Spacer(minLength: 12); actions }
                VStack(alignment: .leading, spacing: 8) { message; actions }
            }
            .padding(.horizontal, KinoTheme.contentPadding)
            .padding(.vertical, 12)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(KinoTheme.surface)
        }
    }

    private var message: some View {
        VStack(alignment: .leading, spacing: 4) {
            Label(session.connection.title, systemImage: "wifi.slash").font(.headline)
            #if os(iOS)
            Text("Saved library content is still available. Play completed downloads without a connection.")
                .font(.callout).foregroundStyle(KinoTheme.muted)
            #else
            Text("Check your network and that your Server is running. Apple TV needs a connection to play.")
                .font(.callout).foregroundStyle(KinoTheme.muted)
            #endif
        }.fixedSize(horizontal: false, vertical: true)
    }

    private var actions: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: 12) { buttons }
            VStack(alignment: .leading, spacing: 8) { buttons }
        }
    }

    @ViewBuilder private var buttons: some View {
        #if os(iOS)
        Button(action: openDownloads) {
            Label("Downloads", systemImage: "arrow.down.circle").frame(minHeight: 44)
        }.buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
        #endif
        Button {
            guard let client = session.client else { return }
            Task { await session.connection.check(client, force: true) }
        } label: {
            Text(session.connection.checking ? "Checking…" : "Try again").frame(minHeight: 44)
        }
        .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
        .disabled(session.connection.checking)
    }
}
