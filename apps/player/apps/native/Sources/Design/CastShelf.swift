import SwiftUI

struct CastShelf: View {
    let people: [CastMember]
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @ScaledMetric(relativeTo: .body) private var cardWidth = 140.0

    var body: some View {
        if !people.isEmpty {
            VStack(alignment: .leading, spacing: 16) {
                Text("Cast").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                if dynamicTypeSize.isAccessibilitySize {
                    LazyVStack(alignment: .leading, spacing: 20) { cards }
                } else {
                    ScrollView(.horizontal) {
                        LazyHStack(alignment: .top, spacing: 18) { cards }
                            .scrollTargetLayout()
                            #if os(tvOS)
                            .padding(.vertical, 24)
                            .focusSection()
                            #endif
                    }
                    .scrollTargetBehavior(.viewAligned)
                    #if os(tvOS)
                    .scrollClipDisabled()
                    #else
                    .scrollBounceBehavior(.basedOnSize, axes: .horizontal)
                    .contentShape(.interaction, .rect)
                    #endif
                }
            }
        }
    }

    private var cards: some View {
        ForEach(Array(people.enumerated()), id: \.offset) { _, person in
            NavigationLink(value: ScreenDestination.actor(person.name)) {
                VStack(alignment: .leading, spacing: 8) {
                    if dynamicTypeSize.isAccessibilitySize {
                        Text(person.name).font(.headline).foregroundStyle(KinoTheme.text)
                        if !person.role.isEmpty { Text(person.role).font(.caption).foregroundStyle(KinoTheme.muted) }
                    } else {
                        Artwork(path: person.image, symbol: "person.fill", dimension: 800)
                            .clipShape(.rect(cornerRadius: 12))
                        Text(person.name).font(.headline).foregroundStyle(KinoTheme.text)
                            .lineLimit(2, reservesSpace: true)
                        Text(person.role.isEmpty ? " " : person.role).font(.caption).foregroundStyle(KinoTheme.muted)
                            .lineLimit(2, reservesSpace: true).accessibilityHidden(person.role.isEmpty)
                    }
                }
                .fixedSize(horizontal: false, vertical: true)
                .frame(width: dynamicTypeSize.isAccessibilitySize ? nil : width, alignment: .leading)
                .frame(maxWidth: dynamicTypeSize.isAccessibilitySize ? .infinity : nil, minHeight: 44, alignment: .leading)
                .contentShape(.rect)
            }
            #if os(iOS)
            .buttonStyle(.plain)
            #else
            .buttonStyle(.card)
            #endif
            .accessibilityElement(children: .combine)
            .accessibilityHint("Shows titles in your library featuring this actor")
        }
    }

    private var width: CGFloat {
        #if os(tvOS)
        230
        #else
        min(cardWidth, dynamicTypeSize.isAccessibilitySize ? 280 : 200)
        #endif
    }
}
