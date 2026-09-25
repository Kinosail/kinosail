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
