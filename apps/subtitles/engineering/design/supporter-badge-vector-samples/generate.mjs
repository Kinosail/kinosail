import { mkdir, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const directory = fileURLToPath(new URL(".", import.meta.url));

const point = (radius, angle) => {
  const radians = (angle * Math.PI) / 180;
  return `${(Math.cos(radians) * radius).toFixed(1)},${(Math.sin(radians) * radius).toFixed(1)}`;
};

const radialPoints = (points, outer, inner, rotation = -90) => Array.from({ length: points * 2 }, (_, index) => {
  const radius = index % 2 ? inner : outer;
  return point(radius, rotation + (index * 180) / points);
}).join(" ");

const ringDots = (count, radius, dotRadius = 2.5, offset = -90) => Array.from({ length: count }, (_, index) => {
  const [x, y] = point(radius, offset + (index * 360) / count).split(",");
  return `<circle class="detail-fill" cx="${x}" cy="${y}" r="${dotRadius}"/>`;
}).join("");

const spokes = (count, inner, outer, offset = -90) => Array.from({ length: count }, (_, index) => {
  const angle = offset + (index * 360) / count;
  const start = point(inner, angle);
  const end = point(outer, angle);
  return `<path class="detail" d="M${start}L${end}"/>`;
}).join("");

const caption = `
  <path class="caption" d="M-29-17h58v37H8L-10 31V20h-19Z"/>
  <path class="caption-lines" d="M-17-5h34M-17 8H9"/>`;

const signalPennants = (level) => {
  const flags = Math.min(6, 2 + Math.floor(level / 2));
  const flagRing = Array.from({ length: flags }, (_, index) => {
    const angle = -90 + (index * 360) / flags;
    const long = 42 + Math.min(level, 6);
    return `<g transform="rotate(${angle})"><path class="detail" d="M0-25V-${long}"/><path class="detail-fill" d="M0-${long}h${13 + level}l-${(5 + level / 3).toFixed(1)} 8H0Z"/></g>`;
  }).join("");
  return `<polygon class="body" points="${radialPoints(4 + Math.floor(level / 3), 55, 45, -90)}"/>${flagRing}<circle class="inner" r="35"/>`;
};

const harborShields = (level) => {
  const width = 38 + level * 1.6;
  const notch = level >= 7 ? 10 : 0;
  const wings = level >= 6
    ? `<path class="detail-fill" d="M-${width - 3}-20h-${12 + level * 1.5}l8 12-${12 + level} 9 20 7 8 16 8-23ZM${width - 3}-20h${12 + level * 1.5}l-8 12 ${12 + level} 9-20 7-8 16-8-23Z"/>`
    : "";
  const crown = level >= 9 ? `<path class="detail" d="M-24-50l10-14L0-50l14-14 10 14"/>` : "";
  return `<path class="body" d="M${-width} ${-(45 - notch)}L${-(18 + notch)} -53 0 ${-(45 + notch)} ${18 + notch} -53 ${width} ${-(45 - notch)} ${width - 5} 15Q${width - 8} 43 0 58Q${-(width - 8)} 43 ${-(width - 5)} 15Z"/>${wings}${crown}<path class="inner" d="M${-(width - 11)} -34 0 -43 ${width - 11} -34 ${width - 14} 14Q${width - 16} 34 0 47Q${-(width - 16)} 34 ${-(width - 14)} 14Z"/>`;
};

const compassOrders = (level) => {
  const points = 4 + level;
  const axes = level >= 5 ? spokes(level >= 9 ? 8 : 4, 39, 55, -90) : "";
  return `<polygon class="body" points="${radialPoints(points, 58, 36 + Math.min(level, 8), -90)}"/>${axes}<circle class="inner" r="34"/><polygon class="detail-fill" points="${radialPoints(4, 31, 12, -90)}"/>`;
};

const portholeMedals = (level) => {
  const bolts = 3 + level;
  const outer = level <= 3
    ? `<circle class="body" r="55"/>`
    : `<polygon class="body" points="${radialPoints(4 + level, 58, level >= 7 ? 45 : 50, -90)}"/>`;
  const ribbons = level >= 7
    ? `<path class="detail-fill" d="M-31 39l-7 24 17-8 12 15 7-31ZM31 39l7 24-17-8-12 15-7-31Z"/>`
    : "";
  const crown = level >= 9 ? `<path class="detail" d="M-27-47l9-13L0-48l18-12 9 13"/>` : "";
  return `${ribbons}${outer}${crown}<circle class="inner" r="43"/><circle class="detail" r="36"/>${ringDots(bolts, 49, level >= 9 ? 3 : 2.4)}`;
};

const beaconStandards = (level) => {
  const housing = 17 + level;
  const rayLength = 42 + level * 2;
  const beams = Array.from({ length: Math.min(4, Math.ceil(level / 2)) }, (_, index) => {
    const y = -34 + index * 13;
    const end = rayLength - index * 2;
    return `<path class="detail" d="M-${housing} ${y}H-${end}M${housing} ${y}H${end}"/>`;
  }).join("");
  const wings = level >= 7 ? `<path class="detail-fill" d="M-${housing + 3} 18h-${10 + level}l8 10-${11 + level} 8 20 8M${housing + 3} 18h${10 + level}l-8 10 ${11 + level} 8-20 8"/>` : "";
  const crown = level >= 9 ? `<path class="detail" d="M-23-54l9-12L0-55l14-11 9 12"/>` : "";
  return `${beams}<path class="body" d="M-${housing}-50h${housing * 2}l9 18-7 8 16 77H-${housing + 18}l16-77-7-8Z"/>${wings}${crown}<path class="inner" d="M-${housing - 2} 44h${(housing - 2) * 2}L15 17h-30ZM-${housing}-34h${housing * 2}l-4 18h-${housing * 2 - 8}Z"/>`;
};

const helmOrders = (level) => {
  const handleCount = 3 + level;
  const handles = Array.from({ length: handleCount }, (_, index) => {
    const angle = -90 + (index * 360) / handleCount;
    const [x, y] = point(58, angle).split(",");
    return `<g transform="rotate(${angle})"><path class="detail" d="M0-35V-54"/></g><circle class="detail-fill" cx="${x}" cy="${y}" r="${level >= 8 ? 4 : 3}"/>`;
  }).join("");
  const wings = level >= 9 ? `<path class="detail" d="M-62-4h18M62-4H44M-59 8h16M59 8H43"/>` : "";
  return `${handles}${wings}<circle class="body" r="46"/><circle class="inner" r="36"/>`;
};

const wakeCrests = (level) => {
  const width = 41 + level * 1.4;
  const waves = Array.from({ length: Math.min(4, 1 + Math.floor(level / 3)) }, (_, index) => {
    const y = 27 + index * 8;
    return `<path class="detail" d="M-42 ${y}q10-9 20 0t20 0t20 0t20 0"/>`;
  }).join("");
  const fins = level >= 4 ? `<path class="detail-fill" d="M-${width - 2}-14l-${8 + level} ${12 + level / 2} ${13 + level} 7M${width - 2}-14l${8 + level} ${12 + level / 2}-${13 + level} 7"/>` : "";
  const pennants = level >= 7 ? `<path class="detail" d="M-24-42v-12l12 5-12 5M24-42v-12l-12 5 12 5"/>` : "";
  const crown = level >= 9 ? `<path class="detail" d="M-29-44l10-14L0-45l19-13 10 14"/>` : "";
  const keel = level >= 7 ? `Q-26 40-8 55L0 48l8 7Q26 40` : `Q-${width - 8} 40 0 ${54 + level / 2}Q${width - 8} 40`;
  return `<path class="body" d="M-${width}-35Q-${width / 2}-${50 + level} 0-39Q${width / 2}-${50 + level} ${width}-35L${width + 5} 8${keel} ${width + 5} 8Z"/>${fins}${pennants}${crown}<path class="inner" d="M-37-29Q-18-${39 + level / 2} 0-31Q18-${39 + level / 2} 37-29L41 9Q32 32 0 46Q-32 32-41 9Z"/>${waves}`;
};

const fleetChevrons = (level) => {
  const width = 39 + level * 1.4;
  const count = Math.min(5, 1 + Math.floor(level / 2));
  const bars = Array.from({ length: count }, (_, index) => {
    const y = 31 + index * 7;
    return `<path class="detail" d="M-${30 + index * 3} ${y}L0 ${y + 12} ${30 + index * 3} ${y}"/>`;
  }).join("");
  const wings = level >= 6 ? `<path class="detail-fill" d="M-43-26l-${10 + level} 5 8 11-${14 + level} 8 18 7-8 13 17-7M43-26l${10 + level} 5-8 11 ${14 + level} 8-18 7 8 13-17-7"/>` : "";
  const star = level >= 9 ? `<polygon class="detail-fill" points="${radialPoints(5, 14, 6, -90)}" transform="translate(0 -47)"/>` : "";
  return `<path class="body" d="M${-width} -48H${width}L${width + 10} -1 0 59 ${-(width + 10)} -1Z"/>${wings}${star}<path class="inner" d="M-31-37H31L39-3 0 45-39-3Z"/>${bars}`;
};

const constellationSeals = (level) => {
  const points = 5 + level;
  const dots = ringDots(Math.min(12, 3 + level), 48, 2.2, -90 + level * 3);
  const orbit = level >= 5 ? `<ellipse class="detail" rx="55" ry="22" transform="rotate(${15 + level * 3})"/>` : "";
  const secondOrbit = level >= 8 ? `<ellipse class="detail" rx="54" ry="28" transform="rotate(${-28 - level})"/>` : "";
  return `<polygon class="body" points="${radialPoints(points, 56, 44, -90)}"/>${orbit}${secondOrbit}${dots}<circle class="inner" r="34"/>`;
};

const sovereignRegalia = (level) => {
  const points = 5 + Math.floor(level / 2);
  const wings = level >= 4 ? `<path class="detail-fill" d="M-36-7l-26-16 8 18-13 8 26 12M36-7l26-16-8 18 13 8-26 12"/>` : "";
  const crown = level >= 7 ? `<path class="detail-fill" d="M-28-43l7-17 15 12L0-66l6 18 15-12 7 17-9 8h-38Z"/>` : "";
  const orbit = level >= 9 ? `<circle class="detail" r="51"/>${ringDots(level, 55, 2.5, -90)}` : "";
  return `<polygon class="body" points="${radialPoints(points, 59, 44 - Math.min(level, 8), -90)}"/>${wings}${crown}${orbit}<polygon class="inner" points="${radialPoints(points, 42, 34, -90)}"/>`;
};

const themes = [
  ["signal-pennants", "Signal Pennants", "Naval flags grow into a radial fleet command standard.", signalPennants],
  ["harbor-shields", "Harbor Shields", "A protected caption field gains guards, wings, and a crown.", harborShields],
  ["compass-orders", "Compass Orders", "Increasing compass resolution makes rank readable by silhouette.", compassOrders],
  ["porthole-medals", "Porthole Medals", "Riveted enamel hardware becomes formal medal regalia.", portholeMedals],
  ["beacon-standards", "Beacon Standards", "Lighthouse optics and signal rays build a vertical order.", beaconStandards],
  ["helm-orders", "Helm Orders", "Wheel handles accumulate into a recognizable fleet command seal.", helmOrders],
  ["wake-crests", "Wake Crests", "Layered wakes sit inside a protective maritime crest.", wakeCrests],
  ["fleet-chevrons", "Fleet Chevrons", "Service stripes rise into wings and senior command stars.", fleetChevrons],
  ["constellation-seals", "Constellation Seals", "Orbital marks turn navigation into a celestial badge system.", constellationSeals],
  ["sovereign-regalia", "Sovereign Regalia", "Crown, wings, and star geometry reserve drama for top levels.", sovereignRegalia],
];

const badge = (family, level, x, y, geometry) => {
  const elite = level >= 7 ? `\n  <text class="elite" x="0" y="99" text-anchor="middle">ELITE</text>` : "";
  const familyLabel = family === "living" ? "Living Standard" : "Patron Order";
  return `<g class="${family}" data-badge="${family}-${level}" data-family="${family}" data-rank="${level}" role="group" aria-label="${familyLabel} level ${level}${level >= 7 ? ", elite" : ""}" transform="translate(${x} ${y})">
  <g aria-hidden="true">${geometry(level)}${caption}</g>
  <text class="rank" x="0" y="82" text-anchor="middle">LEVEL ${String(level).padStart(2, "0")}</text>${elite}
</g>`;
};

const renderSet = ([slug, title, summary, geometry], index) => {
  const columns = Array.from({ length: 10 }, (_, levelIndex) => 94 + levelIndex * 179);
  const living = columns.map((x, levelIndex) => badge("living", levelIndex + 1, x, 297, geometry)).join("\n");
  const patron = columns.map((x, levelIndex) => badge("patron", levelIndex + 1, x, 552, geometry)).join("\n");
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1800 740" role="img" aria-labelledby="title desc">
  <title id="title">${index + 1}. ${title} supporter badge vector set</title>
  <desc id="desc">Ten levels shown in Living Standard signal green and Patron Order brass. The artwork contains SVG geometry only.</desc>
  <style>
    text{font-family:Inter,ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}.eyebrow,.rank,.elite,.family-label,.proof{font-weight:800;letter-spacing:2px}.body{stroke-width:3.5;stroke-linejoin:round}.inner{fill:#090a08;stroke-width:2.5;stroke-linejoin:round}.detail{fill:none;stroke-width:2.4;stroke-linecap:round;stroke-linejoin:round}.detail-fill{stroke:#090a08;stroke-width:1.4;stroke-linejoin:round}.caption{fill:#090a08;stroke-width:3.5;stroke-linejoin:round}.caption-lines{fill:none;stroke-width:5;stroke-linecap:round}.living .body{fill:#18210f;stroke:#c8f169}.living .inner,.living .detail{stroke:#c8f169}.living .detail-fill{fill:#c8f169}.living .caption{stroke:#f6f8ef}.living .caption-lines{stroke:#c8f169}.patron .body{fill:#211a10;stroke:#b49350}.patron .inner,.patron .detail{stroke:#b49350}.patron .detail-fill{fill:#d6b96d}.patron .caption{stroke:#fff4bd}.patron .caption-lines{stroke:#b49350}.rank{fill:#f6f8ef;font-size:11px}.elite{fill:#d6b96d;font-size:9px}.family-label{font-size:13px}.proof{fill:#9ca391;font-size:11px}
  </style>
  <rect width="1800" height="740" fill="#090a08"/>
  <path d="M0 181H1800M0 433H1800M0 685H1800" fill="none" stroke="#2a2d26"/>
  <text class="eyebrow" x="48" y="48" fill="#c8f169" font-size="15">KINOSAIL SUPPORTER BADGE STUDY / SET ${String(index + 1).padStart(2, "0")}</text>
  <text id="set-name" x="48" y="99" fill="#f6f8ef" font-size="45" font-weight="760" letter-spacing="-2">${title}</text>
  <text x="48" y="133" fill="#9ca391" font-size="18">${summary}</text>
  <text class="proof" x="1752" y="48" text-anchor="end">SVG GEOMETRY / ZERO RASTER ASSETS</text>
  <text class="family-label" x="48" y="211" fill="#c8f169">LIVING STANDARD / ACTIVE SIGNAL ENAMEL</text>
  ${living}
  <text class="family-label" x="48" y="463" fill="#d6b96d">PATRON ORDER / PERMANENT BRASS ENAMEL</text>
  ${patron}
  <text x="48" y="718" fill="#9ca391" font-size="13">Same caption emblem. Ten distinct rank silhouettes. Levels 7–10 receive added regalia.</text>
  <text class="proof" x="1752" y="718" text-anchor="end">OPEN AND EDIT AS SVG</text>
</svg>\n`;
};

await mkdir(directory, { recursive: true });
await Promise.all(themes.map((theme, index) => writeFile(`${directory}${String(index + 1).padStart(2, "0")}-${theme[0]}.svg`, renderSet(theme, index))));
