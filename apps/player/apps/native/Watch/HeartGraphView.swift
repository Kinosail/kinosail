import SwiftUI

struct HeartGraphView: View {
    @Environment(MovieHeartTracker.self) private var heart
    let isVisible: Bool
    private let signal = Color(red: 0.77, green: 1, blue: 0.28)

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                if let timeline = heart.timeline {
                    Text("Heart graph").font(.caption.weight(.semibold)).foregroundStyle(signal)
                    Text(timeline.title).font(.headline).lineLimit(2)
                    if heart.loading { ProgressView("Reading Health…") }
                    else if heart.points.isEmpty {
                        Label("No readings yet", systemImage: "heart")
                            .font(.caption.weight(.semibold))
                        Text("Readings appear at their movie time as you watch.")
                            .font(.caption2).foregroundStyle(.secondary)
                    } else {
                        HeartPlot(points: heart.points, duration: timeline.duration, color: signal)
                            .frame(height: 104)
                            .accessibilityLabel("Heart rate across the movie")
                            .accessibilityValue("\(heart.points.count) samples, \(Int(heart.points.map(\.bpm).min() ?? 0)) to \(Int(heart.points.map(\.bpm).max() ?? 0)) beats per minute")
                        HStack { Text("0:00"); Spacer(); Text(clock(timeline.duration)) }
                            .font(.caption2.monospacedDigit()).foregroundStyle(.secondary)
                        Text("\(heart.points.count) readings · \(Int(heart.points.map(\.bpm).min() ?? 0))–\(Int(heart.points.map(\.bpm).max() ?? 0)) bpm")
                            .font(.caption.weight(.semibold))
                    }
                    Text("Gaps mean no reading or movie position was available.")
                        .font(.caption2).foregroundStyle(.secondary)
                    if let message = heart.message {
                        Text(message).font(.caption2).foregroundStyle(.secondary)
                    }
                    if heart.isTracking {
                        Button("Stop tracking") { Task { await heart.stop() } }.buttonStyle(.bordered)
                    }
                }
            }
            .padding(.horizontal, 8)
        }
        .task(id: isVisible) {
            guard isVisible else { return }
            while !Task.isCancelled {
                await heart.loadSamples()
                guard heart.isTracking else { return }
                try? await Task.sleep(for: .seconds(20))
            }
        }
    }
}

private struct HeartPlot: View {
    let points: [HeartPoint]
    let duration: Double
    let color: Color

    var body: some View {
        Canvas { context, size in
            guard duration > 0, let low = points.map(\.bpm).min(), let high = points.map(\.bpm).max() else { return }
            let floor = max(25, low - 10), range = max(20, high - floor + 10)
            var path = Path()
            var previous: HeartPoint?
            for point in points {
                let location = CGPoint(x: size.width * point.position / duration,
                                       y: size.height * (1 - (point.bpm - floor) / range))
                if let previous, point.time.timeIntervalSince(previous.time) <= 30,
                   point.position >= previous.position, point.position - previous.position <= 30 {
                    path.addLine(to: location)
                } else { path.move(to: location) }
                context.fill(Path(ellipseIn: CGRect(x: location.x - 2, y: location.y - 2, width: 4, height: 4)), with: .color(color))
                previous = point
            }
            context.stroke(path, with: .color(color), style: StrokeStyle(lineWidth: 2, lineCap: .round))
        }
    }
}
