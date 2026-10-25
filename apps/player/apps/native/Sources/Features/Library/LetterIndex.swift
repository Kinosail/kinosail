import SwiftUI

#if os(iOS)
struct LetterScrubber: View {
    let letters: [LibraryPage.Letter]
    let onSelect: (LibraryPage.Letter) -> Void
    @State private var active: Int?
    @State private var sent: Int?
    @State private var spoken = 0
    @ScaledMetric(relativeTo: .caption2) private var preferredRow: CGFloat = 16

    var body: some View {
        GeometryReader { proxy in
            let row = max(1, min(preferredRow, (proxy.size.height - 16) / CGFloat(max(letters.count, 1))))
            VStack(spacing: 0) {
                ForEach(Array(letters.enumerated()), id: \.element.id) { index, letter in
                    Text(letter.label)
                        .font(.caption2.weight(.bold))
                        .minimumScaleFactor(0.6)
                        .foregroundStyle(active == index ? KinoTheme.signal : KinoTheme.text.opacity(0.82))
                        .frame(width: 30, height: row)
                }
            }
            .contentShape(Rectangle())
            .gesture(DragGesture(minimumDistance: 0)
                .onChanged { active = min(max(Int($0.location.y / row), 0), letters.count - 1) }
                .onEnded { _ in
                    if let active, active != sent { onSelect(letters[active]) }
                    active = nil; sent = nil
                })
            .overlay(alignment: .topTrailing) {
                if let active {
                    Text(letters[active].label)
                        .font(.largeTitle.weight(.bold))
                        .foregroundStyle(KinoTheme.text)
                        .frame(width: 72, height: 72)
                        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 20, style: .continuous))
                        .offset(x: -64, y: row * (CGFloat(active) + 0.5) - 36)
                        .allowsHitTesting(false)
                        .transition(.scale(scale: 0.8).combined(with: .opacity))
                }
            }
            .padding(.vertical, 8)
            .background(.ultraThinMaterial.opacity(active == nil ? 0 : 1), in: Capsule())
            .animation(.snappy(duration: 0.16), value: active)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .trailing)
        }
        .frame(width: 34)
        .padding(.vertical, 12)
        .sensoryFeedback(.selection, trigger: active) { _, new in new != nil }
        .task(id: active) {
            guard let active, active != sent else { return }
            try? await Task.sleep(for: .milliseconds(180))
            guard !Task.isCancelled else { return }
            sent = active
            onSelect(letters[active])
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Title index")
        .accessibilityValue(letters.indices.contains(spoken) ? letters[spoken].label : "")
        .accessibilityHint("Swipe up or down to jump to a letter")
        .accessibilityAdjustableAction { direction in
            switch direction {
            case .increment: spoken = min(spoken + 1, letters.count - 1)
            case .decrement: spoken = max(spoken - 1, 0)
            @unknown default: return
            }
            if letters.indices.contains(spoken) { onSelect(letters[spoken]) }
        }
    }
}
#endif

#if os(tvOS)
struct TVLetterIndex: View {
    let letters: [LibraryPage.Letter]
    let onFocus: (String?) -> Void
    let onBrowse: (LibraryPage.Letter) -> Void
    @FocusState private var focusedLetter: String?

    var body: some View {
        GeometryReader { proxy in
            let row = min(48, (proxy.size.height - 28) / CGFloat(max(letters.count, 1)))
            VStack(spacing: 0) {
                ForEach(letters) { letter in
                    Button(letter.label) { onBrowse(letter) }
                        .tvFlat()
                        .font(.system(size: min(29, row * 0.8), weight: .semibold))
                        .frame(width: 60, height: row)
                        .foregroundStyle(focusedLetter == letter.label ? KinoTheme.tvOSPrimaryInk : KinoTheme.text)
                        .background(focusedLetter == letter.label ? .white : .clear, in: Capsule())
                        .animation(.snappy(duration: 0.18), value: focusedLetter)
                        .focused($focusedLetter, equals: letter.label)
                        .accessibilityLabel("\(letter.label), \(letter.count) \(letter.count == 1 ? "title" : "titles")")
                        .accessibilityHint("Focus to browse this letter")
                }
            }
            .padding(.vertical, 14)
            .frame(width: 88)
            .background(KinoTheme.surface, in: RoundedRectangle(cornerRadius: 14))
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        }
        .frame(width: 88)
        .padding(.top, 96)
        .padding(.bottom, 32)
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
