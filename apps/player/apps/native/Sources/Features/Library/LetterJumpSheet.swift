import SwiftUI

struct LetterJumpSheet: View {
    let letters: [LibraryPage.Letter]
    let onSelect: (LibraryPage.Letter) -> Void
    @Environment(\.dismiss) private var dismiss
    private var buttonWidth: CGFloat {
        #if os(tvOS)
        140
        #else
        76
        #endif
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Text("Choose a starting letter. Titles stay sorted A–Z.")
                        .font(.subheadline)
                        .foregroundStyle(KinoTheme.muted)

                    LazyVGrid(columns: [GridItem(.adaptive(minimum: buttonWidth), spacing: 12)], spacing: 12) {
                        ForEach(letters) { letter in
                            Button {
                                onSelect(letter)
                                dismiss()
                            } label: {
                                VStack(spacing: 4) {
                                    Text(letter.label)
                                        .font(.title3.weight(.semibold))
                                    Text("\(letter.count) \(letter.count == 1 ? "title" : "titles")")
                                        .font(.caption)
                                        .foregroundStyle(KinoTheme.muted)
                                }
                                .frame(maxWidth: .infinity, minHeight: 72)
                            }
                            #if os(tvOS)
                            .buttonStyle(.card)
                            #else
                            .buttonStyle(.bordered)
                            .buttonBorderShape(.roundedRectangle(radius: 14))
                            .tint(KinoTheme.secondaryControlTint)
                            .secondaryControlForeground()
                            #endif
                        }
                    }
                }
                .padding(KinoTheme.contentPadding)
            }
            .background(KinoTheme.background)
            .navigationTitle("Browse by title")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Done") { dismiss() }
                }
            }
        }
        #if os(iOS)
        .presentationDetents([.medium, .large])
        .presentationDragIndicator(.visible)
        #endif
    }
}

#if os(tvOS)
struct TVLetterIndex: View {
    let letters: [LibraryPage.Letter]
    let onFocus: (String?) -> Void
    let onBrowse: (LibraryPage.Letter) -> Void
    @FocusState private var focusedLetter: String?

    var body: some View {
        ScrollView(.vertical) {
            LazyVStack(spacing: 2) {
                ForEach(letters) { letter in
                    Button(letter.label) { onBrowse(letter) }
                        .buttonStyle(.plain)
                        .font(.callout.weight(.semibold))
                        .frame(width: 56, height: 44)
                        .foregroundStyle(focusedLetter == letter.label ? KinoTheme.signalInk : KinoTheme.text)
                        .background(focusedLetter == letter.label ? KinoTheme.signal : .clear,
                                    in: RoundedRectangle(cornerRadius: 10))
                        .focused($focusedLetter, equals: letter.label)
                        .accessibilityLabel("\(letter.label), \(letter.count) \(letter.count == 1 ? "title" : "titles")")
                        .accessibilityHint("Focus to browse this letter")
                }
            }
            .padding(.vertical, 8)
        }
        .scrollIndicators(.hidden)
        .frame(width: 68)
        .frame(maxHeight: 680)
        .background(KinoTheme.surface, in: RoundedRectangle(cornerRadius: 14))
        .focusSection()
        .onChange(of: focusedLetter) { _, letter in onFocus(letter) }
        .task(id: focusedLetter) {
            guard let focusedLetter, let letter = letters.first(where: { $0.label == focusedLetter }) else { return }
            try? await Task.sleep(for: .milliseconds(150))
            guard !Task.isCancelled else { return }
            onBrowse(letter)
        }
    }
}
#endif
