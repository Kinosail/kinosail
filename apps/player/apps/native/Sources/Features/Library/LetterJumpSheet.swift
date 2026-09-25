import SwiftUI

#if os(iOS)
struct LetterJumpSheet: View {
    let letters: [LibraryPage.Letter]
    let onSelect: (LibraryPage.Letter) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Text("Choose a starting letter. Titles stay sorted A–Z.")
                        .font(.subheadline)
                        .foregroundStyle(KinoTheme.muted)

                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 76), spacing: 12)], spacing: 12) {
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
                            .buttonStyle(.bordered)
                            .buttonBorderShape(.roundedRectangle(radius: 14))
                            .tint(KinoTheme.secondaryControlTint)
                            .foregroundStyle(KinoTheme.text)
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
        .presentationDetents([.medium, .large])
        .presentationDragIndicator(.visible)
    }
}
#endif
