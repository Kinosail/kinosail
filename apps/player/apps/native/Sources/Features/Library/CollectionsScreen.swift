import SwiftUI

struct CollectionsScreen: View {
    @Environment(AppSession.self) private var session
    var body: some View {
        ScrollView {
            ResourceView(identity: session.profileKey ?? "", load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.collections(policy: policy)
            }) { names in
                if names.isEmpty {
                    FeaturePlaceholder(title: "No collections yet", symbol: "rectangle.stack", message: "Collections from your Server will appear here.")
                } else {
                    LazyVStack(alignment: .leading, spacing: 20) {
                        ForEach(names, id: \.self) { name in
                            NavigationLink(value: ScreenDestination.collection(name)) {
                                Label(name, systemImage: "rectangle.stack").font(.title3)
                                    .frame(maxWidth: .infinity, minHeight: 48, alignment: .leading)
                            }
                            Divider()
                        }
                    }
                }
            }.padding(KinoTheme.contentPadding)
        }
        .navigationTitle("Collections")
    }
}

struct CollectionScreen: View {
    let name: String
    @Environment(AppSession.self) private var session
    var body: some View {
        ScrollView {
            ResourceView(identity: name, load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.collection(name: name, policy: policy)
            }) { items in
                if items.isEmpty { FeaturePlaceholder(title: "No titles yet", symbol: "rectangle.stack", message: "This collection is empty.") }
                else { MediaGrid(items: items) }
            }.padding(KinoTheme.contentPadding)
        }
        .navigationTitle(name)
    }
}
