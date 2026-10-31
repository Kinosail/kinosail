import SwiftUI

#if os(tvOS)
struct ShelfStartPosition: ViewModifier {
    let itemID: String?
    let focusedCard: FocusState<String?>.Binding
    @State private var position: String?

    func body(content: Content) -> some View {
        if let itemID {
            content
                .scrollPosition(id: $position, anchor: .leading)
                .defaultFocus(focusedCard, itemID, priority: .userInitiated)
                .onAppear { if position == nil { position = itemID } }
        } else {
            content
        }
    }
}
#endif

#if os(tvOS)
struct ShelfCardFocus: ViewModifier {
    let active: Bool
    let id: String
    let focusedCard: FocusState<String?>.Binding

    func body(content: Content) -> some View {
        if active { content.focused(focusedCard, equals: id) } else { content }
    }
}
#endif
