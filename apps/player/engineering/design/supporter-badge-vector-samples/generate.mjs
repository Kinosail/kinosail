import { mkdir, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const outputDirectory = dirname(fileURLToPath(import.meta.url));
const ranks = [
  "Friend",
  "Crew",
  "Navigator",
  "Patron",
  "Steward",
  "Lighthouse",
  "Commodore",
  "Admiral",
  "North Star",
  "Legacy",
];

const rankShapes = ["coin", "hex", "lozenge", "shield", "medal", "standard", "compass", "burst", "crest", "sovereign"];

const directions = [
  { slug: "01-tideglass-orbit", name: "Tideglass Orbit", note: "Sea glass, orbital brass, living current", shape: "orbit", field: "#071b18", field2: "#123b34", metal1: "#fff2c8", metal2: "#aa7440", accent: "#55e2bd", jewel: "#a6ffdc" },
  { slug: "02-admiralty-seal", name: "Admiralty Seal", note: "Onyx medal, rope edge, permanent authority", shape: "coin", field: "#090a08", field2: "#282116", metal1: "#fff0b8", metal2: "#8f5e20", accent: "#d6b96d", jewel: "#47c88a" },
  { slug: "03-compass-rose", name: "Compass Rose", note: "Ivory points, emerald bearings, exact navigation", shape: "compass", field: "#0c120f", field2: "#203128", metal1: "#fff8df", metal2: "#b58b48", accent: "#c8f169", jewel: "#27d49a" },
  { slug: "04-harbor-guard", name: "Harbor Guard", note: "Bronze shield, deep navy, protected passage", shape: "shield", field: "#071018", field2: "#172d3a", metal1: "#f2cf92", metal2: "#86542f", accent: "#7fc6d4", jewel: "#c8f169" },
  { slug: "05-lighthouse-order", name: "Lighthouse Order", note: "Signal lime, graphite iron, visible command", shape: "arch", field: "#090b08", field2: "#20281b", metal1: "#f4f7e9", metal2: "#68745d", accent: "#c8f169", jewel: "#f4ffb5" },
  { slug: "06-celestial-fleet", name: "Celestial Fleet", note: "Midnight enamel, silver orbit, quiet precision", shape: "celestial", field: "#080b18", field2: "#19274d", metal1: "#eef5ff", metal2: "#7383a7", accent: "#78b8ff", jewel: "#c8f169" },
  { slug: "07-deep-current", name: "Deep Current", note: "Copper current, dark teal, carved wave mass", shape: "drop", field: "#061412", field2: "#17433d", metal1: "#ffd4a0", metal2: "#9b5634", accent: "#52d4c3", jewel: "#f2ca72" },
  { slug: "08-royal-wake", name: "Royal Wake", note: "Oxblood enamel, old gold, ceremonial passage", shape: "oval", field: "#1a080a", field2: "#482126", metal1: "#fff0bb", metal2: "#9d5e27", accent: "#e7ba65", jewel: "#53c88e" },
  { slug: "09-cartographers-guild", name: "Cartographer's Guild", note: "Vellum brass, ink grid, measured discovery", shape: "lozenge", field: "#17150d", field2: "#39351f", metal1: "#fff6cf", metal2: "#9d7b3d", accent: "#d4c77c", jewel: "#7ce0b7" },
  { slug: "10-legacy-sovereign", name: "Legacy Sovereign", note: "Obsidian regalia, auric frame, final-rank gravity", shape: "sovereign", field: "#070806", field2: "#252016", metal1: "#fff4bd", metal2: "#a56b20", accent: "#e4cc83", jewel: "#28d68f" },
];

function starPoints(points, outer, inner, rotation = -Math.PI / 2) {
  return Array.from({ length: points * 2 }, (_, index) => {
    const radius = index % 2 ? inner : outer;
    const angle = rotation + index * Math.PI / points;
    return `${(Math.cos(angle) * radius).toFixed(2)},${(Math.sin(angle) * radius).toFixed(2)}`;
  }).join(" ");
}

function polygonPoints(points, radius, rotation = -Math.PI / 2) {
  return Array.from({ length: points }, (_, index) => {
    const angle = rotation + index * Math.PI * 2 / points;
    return `${(Math.cos(angle) * radius).toFixed(2)},${(Math.sin(angle) * radius).toFixed(2)}`;
  }).join(" ");
}

function silhouette(shape, scale = 1, attributes = "") {
  const scaled = (value) => (value * scale).toFixed(2);
  switch (shape) {
    case "hex":
      return `<polygon points="${polygonPoints(6, 124 * scale, -Math.PI / 2)}" ${attributes}/>`;
    case "compass":
      return `<polygon points="${starPoints(8, 126 * scale, 104 * scale)}" ${attributes}/>`;
    case "burst":
      return `<polygon points="${starPoints(14, 137 * scale, 108 * scale)}" ${attributes}/>`;
    case "crest":
      return `<path d="M${scaled(-112)} ${scaled(-79)}L${scaled(-74)} ${scaled(-103)}L${scaled(-46)} ${scaled(-91)}L${scaled(-20)} ${scaled(-126)}L0 ${scaled(-99)}L${scaled(20)} ${scaled(-126)}L${scaled(46)} ${scaled(-91)}L${scaled(74)} ${scaled(-103)}L${scaled(112)} ${scaled(-79)}V${scaled(26)}C${scaled(112)} ${scaled(91)} ${scaled(59)} ${scaled(125)} 0 ${scaled(143)}C${scaled(-59)} ${scaled(125)} ${scaled(-112)} ${scaled(91)} ${scaled(-112)} ${scaled(26)}Z" ${attributes}/>`;
    case "shield":
      return `<path d="M0 ${scaled(-124)}L${scaled(105)} ${scaled(-86)}V${scaled(22)}C${scaled(105)} ${scaled(82)} ${scaled(58)} ${scaled(116)} 0 ${scaled(139)}C${scaled(-58)} ${scaled(116)} ${scaled(-105)} ${scaled(82)} ${scaled(-105)} ${scaled(22)}V${scaled(-86)}Z" ${attributes}/>`;
    case "arch":
      return `<path d="M${scaled(-102)} ${scaled(132)}V${scaled(-28)}C${scaled(-102)} ${scaled(-96)} ${scaled(-58)} ${scaled(-132)} 0 ${scaled(-132)}S${scaled(102)} ${scaled(-96)} ${scaled(102)} ${scaled(-28)}V${scaled(132)}L0 ${scaled(104)}Z" ${attributes}/>`;
    case "drop":
      return `<path d="M0 ${scaled(-136)}C${scaled(26)} ${scaled(-96)} ${scaled(111)} ${scaled(-38)} ${scaled(111)} ${scaled(43)}C${scaled(111)} ${scaled(110)} ${scaled(58)} ${scaled(142)} 0 ${scaled(142)}S${scaled(-111)} ${scaled(110)} ${scaled(-111)} ${scaled(43)}C${scaled(-111)} ${scaled(-38)} ${scaled(-26)} ${scaled(-96)} 0 ${scaled(-136)}Z" ${attributes}/>`;
    case "oval":
      return `<ellipse rx="${scaled(104)}" ry="${scaled(132)}" ${attributes}/>`;
    case "lozenge":
      return `<polygon points="0,${scaled(-138)} ${scaled(116)},0 0,${scaled(138)} ${scaled(-116)},0" ${attributes}/>`;
    case "medal":
      return `<path d="M${scaled(-43)} ${scaled(-133)}H${scaled(43)}L${scaled(31)} ${scaled(-101)}C${scaled(85)} ${scaled(-87)} ${scaled(118)} ${scaled(-42)} ${scaled(118)} ${scaled(13)}C${scaled(118)} ${scaled(81)} ${scaled(65)} ${scaled(132)} 0 ${scaled(132)}S${scaled(-118)} ${scaled(81)} ${scaled(-118)} ${scaled(13)}C${scaled(-118)} ${scaled(-42)} ${scaled(-85)} ${scaled(-87)} ${scaled(-31)} ${scaled(-101)}Z" ${attributes}/>`;
    case "standard":
      return `<path d="M${scaled(-100)} ${scaled(-126)}H${scaled(100)}V${scaled(100)}L${scaled(55)} ${scaled(82)}L0 ${scaled(137)}L${scaled(-55)} ${scaled(82)}L${scaled(-100)} ${scaled(100)}Z" ${attributes}/>`;
    case "sovereign":
      return `<polygon points="${starPoints(20, 139 * scale, 121 * scale)}" ${attributes}/>`;
    default:
      return `<circle r="${scaled(122)}" ${attributes}/>`;
  }
}

function pearls(rank, radius, color) {
  const count = Math.min(12, rank + 2);
  return Array.from({ length: count }, (_, index) => {
    const angle = -Math.PI / 2 + index * Math.PI * 2 / count;
    const x = Math.cos(angle) * radius;
    const y = Math.sin(angle) * radius;
    const size = rank >= 8 && index % 3 === 0 ? 4.2 : 2.6;
    return `<circle cx="${x.toFixed(2)}" cy="${y.toFixed(2)}" r="${size}" fill="${color}"/>`;
  }).join("");
}

function cardinals(rank, gradientID, accent) {
  if (rank < 3) return "";
  const size = Math.min(17, 8 + rank);
  return [0, 90, 180, 270].map((rotation) => `<g transform="rotate(${rotation}) translate(0 -112)"><path d="M0 -${size}L${(size * .42).toFixed(1)} 0L0 ${size}L${(-size * .42).toFixed(1)} 0Z" fill="url(#${gradientID})" stroke="${accent}" stroke-width="1.5"/></g>`).join("");
}

function headpiece(shape, rank, gradientID, jewel, accent) {
  if (rank < 9) return "";
  const upper = rank === 10 ? -162 : -151;
  if (["orbit", "celestial", "compass", "lozenge"].includes(shape)) {
    return `<g transform="translate(0 ${upper})"><polygon points="${starPoints(rank === 10 ? 8 : 4, rank === 10 ? 39 : 32, rank === 10 ? 13 : 10)}" fill="url(#${gradientID})" stroke="${accent}" stroke-width="2"/><circle r="7" fill="${jewel}"/><circle r="3" fill="#090a08"/></g>`;
  }
  if (shape === "arch") {
    return `<g transform="translate(0 ${upper})"><path d="M-52 27L-34-9L0-35L34-9L52 27M-39 10H39M0-35V20" fill="none" stroke="url(#${gradientID})" stroke-width="7"/><circle cy="-35" r="6" fill="${jewel}"/></g>`;
  }
  if (shape === "drop") {
    return `<g transform="translate(0 ${upper})"><path d="M-51 25Q-32-18 0 11Q32-18 51 25Q22 12 0 38Q-22 12-51 25Z" fill="url(#${gradientID})" stroke="${accent}" stroke-width="2"/><path d="M0-30C18-7 18 7 0 22C-18 7-18-7 0-30Z" fill="${jewel}"/></g>`;
  }
  return `<g transform="translate(0 ${upper})"><path d="M-48 22L-39-19L-13 7L0-30L14 7L40-19L49 22Z" fill="url(#${gradientID})" stroke="${jewel}" stroke-width="1.6"/><path d="M-50 24H51L44 35H-43Z" fill="url(#${gradientID})"/><circle cy="-4" r="5" fill="${jewel}"/></g>`;
}

function laurel(rank, gradientID) {
  if (rank < 8) return "";
  const leaves = [];
  for (let index = 0; index < 7; index += 1) {
    const y = 78 - index * 24;
    const x = 109 + Math.sin(index * .7) * 6;
    const rotation = -28 - index * 5;
    leaves.push(`<ellipse cx="${x}" cy="${y}" rx="6" ry="15" transform="rotate(${rotation} ${x} ${y})" fill="url(#${gradientID})"/>`);
    leaves.push(`<ellipse cx="${-x}" cy="${y}" rx="6" ry="15" transform="rotate(${-rotation} ${-x} ${y})" fill="url(#${gradientID})"/>`);
  }
  return `<g opacity=".96">${leaves.join("")}<path d="M-118 100Q-142 0-96-91M118 100Q142 0 96-91" fill="none" stroke="url(#${gradientID})" stroke-width="4"/></g>`;
}

function finalFlourish(shape, rank, gradientID, field, accent, jewel) {
  if (rank < 10) return "";
  if (shape === "orbit" || shape === "celestial") {
    return `<g fill="none" stroke="url(#${gradientID})" stroke-width="5"><ellipse rx="187" ry="69" transform="rotate(-18)"/><ellipse rx="187" ry="69" transform="rotate(18)"/></g><g fill="${jewel}" stroke="${accent}" stroke-width="2"><circle cx="178" cy="-52" r="8"/><circle cx="-178" cy="52" r="8"/><circle cx="178" cy="52" r="5"/><circle cx="-178" cy="-52" r="5"/></g>`;
  }
  if (shape === "compass" || shape === "lozenge" || shape === "sovereign") {
    return `<g fill="url(#${gradientID})" stroke="${accent}" stroke-width="2"><path d="M-111-63L-188 0L-111 63L-139 0Z"/><path d="M111-63L188 0L111 63L139 0Z"/></g><g fill="${jewel}"><circle cx="-171" r="6"/><circle cx="171" r="6"/></g>`;
  }
  if (shape === "shield" || shape === "arch") {
    return `<g fill="${field}" stroke="url(#${gradientID})" stroke-width="5"><path d="M-95-41Q-151-77-181-36L-145-4L-181 32Q-143 58-97 31Z"/><path d="M95-41Q151-77 181-36L145-4L181 32Q143 58 97 31Z"/></g><path d="M-166-33L-108-4M166-33L108-4" stroke="${accent}" stroke-width="3"/>`;
  }
  if (shape === "drop") {
    return `<g fill="none" stroke="url(#${gradientID})" stroke-width="7" stroke-linecap="round"><path d="M-98-24C-143-72-183-45-183-8C-150-34-123-15-112 16C-152-9-180 14-167 47C-141 28-119 45-103 70"/><path d="M98-24C143-72 183-45 183-8C150-34 123-15 112 16C152-9 180 14 167 47C141 28 119 45 103 70"/></g>`;
  }
  const side = (mirror) => `<g transform="scale(${mirror} 1)"><path d="M108-38C150-79 175-66 184-46C157-43 143-28 127-8C166-34 190-16 186 5C160 3 144 16 126 34C164 19 180 37 170 55C146 50 130 61 110 77" fill="${field}" stroke="url(#${gradientID})" stroke-width="5"/><path d="M123-23L171-50M126 9L177 0M121 42L162 50" stroke="url(#${gradientID})" stroke-width="2"/></g>`;
  return side(1) + side(-1);
}

function crossedCommand(rank, gradientID) {
  if (rank < 7) return "";
  const blade = `<path d="M-9-128L0-148L9-128L4 99L17 116L0 139L-17 116L-4 99Z" fill="url(#${gradientID})"/>`;
  return `<g opacity=".88" transform="rotate(45)">${blade}</g><g opacity=".88" transform="rotate(-45)">${blade}</g>`;
}

function lowerHonor(rank, gradientID, jewel) {
  if (rank < 6) return "";
  return `<g transform="translate(0 133)"><path d="M-35-12L-18 39L0 22L18 39L35-12Z" fill="url(#${gradientID})" stroke="${jewel}" stroke-width="1.4"/><path d="M0 14L11 29L0 47L-11 29Z" fill="${jewel}"/></g>`;
}

function signature(shape, rank, gradientID, accent, jewel) {
  if (shape === "orbit" || shape === "celestial") {
    const third = rank >= 5 ? `<ellipse rx="125" ry="52" transform="rotate(55)"/>` : "";
    return `<g fill="none" stroke="${accent}" stroke-width="2" opacity=".7"><ellipse rx="128" ry="58" transform="rotate(-26)"/><ellipse rx="128" ry="58" transform="rotate(26)"/>${third}</g>${rank >= 7 ? `<circle cx="91" cy="-88" r="7" fill="${jewel}"/><circle cx="-105" cy="54" r="5" fill="${jewel}"/>` : ""}`;
  }
  if (shape === "coin") {
    return `<circle r="113" fill="none" stroke="url(#${gradientID})" stroke-width="8" stroke-dasharray="2 6"/>${rank >= 5 ? `<circle r="101" fill="none" stroke="${accent}" stroke-width="1.4"/>` : ""}`;
  }
  if (shape === "compass" || shape === "lozenge") {
    return `<polygon points="${starPoints(4, 124, 21)}" fill="url(#${gradientID})" opacity=".78"/>${rank >= 6 ? `<polygon points="${starPoints(8, 118, 82)}" fill="none" stroke="${accent}" stroke-width="2"/>` : ""}`;
  }
  if (shape === "shield") {
    return `<path d="M0-112V111M-92-42H92" stroke="${accent}" stroke-width="2" opacity=".55"/>${rank >= 7 ? `<path d="M-74 78L74-78M-74-78L74 78" stroke="url(#${gradientID})" stroke-width="4"/>` : ""}`;
  }
  if (shape === "arch") {
    const beams = rank >= 5 ? [-58, -35, 0, 35, 58].map((angle) => `<path d="M0-55L0-121" transform="rotate(${angle})"/>`).join("") : "";
    return `<g fill="none" stroke="${accent}" stroke-width="3" opacity=".72"><path d="M-76 62H76M-68 80H68"/>${beams}</g>`;
  }
  if (shape === "drop") {
    return `<g fill="none" stroke="url(#${gradientID})" stroke-linecap="round"><path d="M-93 57Q-47 27 0 57T93 57" stroke-width="7"/><path d="M-82 80Q-40 53 0 80T82 80" stroke-width="5"/>${rank >= 5 ? `<path d="M-67 101Q-31 79 0 101T67 101" stroke-width="3"/>` : ""}</g>`;
  }
  if (shape === "oval") {
    return `${rank >= 4 ? `<path d="M-92-72Q-132 0-92 79M92-72Q132 0 92 79" fill="none" stroke="url(#${gradientID})" stroke-width="5"/>` : ""}${rank >= 7 ? `<path d="M-55-110Q0-136 55-110" fill="none" stroke="${accent}" stroke-width="3"/>` : ""}`;
  }
  if (shape === "sovereign") {
    return `<polygon points="${starPoints(rank >= 8 ? 16 : 12, 126, rank >= 8 ? 104 : 109)}" fill="none" stroke="url(#${gradientID})" stroke-width="6"/>${rank >= 6 ? `<circle r="108" fill="none" stroke="${accent}" stroke-width="2" stroke-dasharray="1 7"/>` : ""}`;
  }
  return "";
}

function sailMark(config, rank, ids) {
  const halo = rank >= 7 ? `<circle r="72" fill="none" stroke="url(#${ids.metal})" stroke-width="3"/><circle r="66" fill="none" stroke="${config.accent}" stroke-width="1.5"/>` : "";
  return `<g>${halo}<circle r="59" fill="url(#${ids.core})" stroke="url(#${ids.metal})" stroke-width="5"/><path d="M-14 34V-43C11-34 33-13 46 14L2 5Z" fill="url(#${ids.metal})" stroke="${config.metal1}" stroke-width="1.4"/><path d="M-18-31V14L-43 14C-33-2-26-17-18-31Z" fill="${config.accent}" opacity=".88"/><path d="M4-13L31 4L4 22Z" fill="${config.field}"/><path d="M-48 35Q-24 23 0 35T48 35" fill="none" stroke="${config.jewel}" stroke-width="5" stroke-linecap="round"/><path d="M-40 45Q-20 35 0 45T40 45" fill="none" stroke="${config.accent}" stroke-width="2" stroke-linecap="round"/></g>`;
}

function badge(config, rank, x, y) {
  const rankShape = rankShapes[rank - 1];
  const ids = {
    clip: `${config.slug}-clip-${rank}`,
    field: `${config.slug}-field`,
    metal: `${config.slug}-metal`,
    core: `${config.slug}-core`,
    lines: `${config.slug}-lines`,
  };
  const level = String(rank).padStart(2, "0");
  const innerScale = rank >= 9 ? .79 : .83;
  const artScale = (.60 + rank * .022).toFixed(3);
  const ornaments = [
    finalFlourish(config.shape, rank, ids.metal, config.field, config.accent, config.jewel),
    crossedCommand(rank, ids.metal),
    silhouette(rankShape, 1, `fill="#000" opacity=".42" transform="translate(0 9)"`),
    silhouette(rankShape, 1, `fill="url(#${ids.field})" stroke="url(#${ids.metal})" stroke-width="${rank >= 8 ? 13 : 10}" stroke-linejoin="round"`),
    rank >= 2 ? `<g transform="scale(.93)">${silhouette(rankShape, 1, `fill="none" stroke="${config.field2}" stroke-width="4"`)}</g>` : "",
    rank >= 3 ? `<g transform="scale(.88)">${silhouette(rankShape, 1, `fill="none" stroke="url(#${ids.metal})" stroke-width="2.4"`)}</g>` : "",
    rank >= 4 ? `<g transform="scale(${innerScale})">${silhouette(rankShape, 1, `fill="none" stroke="${config.accent}" stroke-width="2.4"`)}</g>` : "",
    rank >= 3 ? `<rect x="-150" y="-150" width="300" height="300" fill="url(#${ids.lines})" clip-path="url(#${ids.clip})" opacity=".7"/>` : "",
    rank >= 3 ? signature(config.shape, rank, ids.metal, config.accent, config.jewel) : "",
    rank >= 2 ? `<circle r="94" fill="none" stroke="url(#${ids.metal})" stroke-width="2"/><circle r="88" fill="none" stroke="${config.accent}" stroke-width="1" opacity=".72"/>` : "",
    rank >= 4 ? `<circle r="103" fill="none" stroke="url(#${ids.metal})" stroke-width="5" stroke-dasharray="1 7" stroke-linecap="round"/>` : "",
    cardinals(rank, ids.metal, config.accent),
    rank >= 4 ? pearls(rank, rank >= 8 ? 122 : 105, config.jewel) : "",
    rank >= 5 ? `<g transform="translate(0 -123)"><path d="M0-19L12 0L0 19L-12 0Z" fill="${config.jewel}" stroke="url(#${ids.metal})" stroke-width="3"/><path d="M0-10L6 0L0 10L-6 0Z" fill="${config.field}"/></g>` : "",
    laurel(rank, ids.metal),
    lowerHonor(rank, ids.metal, config.jewel),
    headpiece(config.shape, rank, ids.metal, config.jewel, config.accent),
    sailMark(config, rank, ids),
  ].join("");
  const serviceDots = Array.from({ length: Math.min(6, rank) }, (_, index) => `<circle cx="${-17.5 * (Math.min(6, rank) - 1) / 2 + index * 17.5}" cy="164" r="3" fill="${index === Math.min(6, rank) - 1 ? config.jewel : config.accent}"/>`).join("");
  return `<g data-badge="level-${rank}" data-rank="${rank}" data-shape="${rankShape}" data-art-scale="${artScale}" role="group" aria-label="${ranks[rank - 1]} badge, level ${rank}" transform="translate(${x} ${y})"><g aria-hidden="true" transform="scale(${artScale})">${ornaments}${serviceDots}</g><text x="0" y="158" text-anchor="middle" class="level">LEVEL ${level}</text><text x="0" y="181" text-anchor="middle" class="rank">${ranks[rank - 1].toUpperCase()}</text></g>`;
}

function sheet(config) {
  const ids = {
    clip: `${config.slug}-clip`,
    field: `${config.slug}-field`,
    metal: `${config.slug}-metal`,
    core: `${config.slug}-core`,
    lines: `${config.slug}-lines`,
  };
  const badges = Array.from({ length: 10 }, (_, index) => {
    const column = index % 5;
    const row = Math.floor(index / 5);
    return badge(config, index + 1, 180 + column * 310, 350 + row * 385);
  }).join("");
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 1000" role="img" aria-labelledby="title description" data-vector-only="true">
<title id="title">${config.name}, ten Kinosail supporter badge levels</title>
<desc id="description">A pure vector concept sheet. Badge detail, size, and ceremony increase from level one through level ten.</desc>
<defs>
  <linearGradient id="${ids.metal}" x1="0" y1="0" x2="1" y2="1"><stop stop-color="${config.metal2}"/><stop offset=".22" stop-color="${config.metal1}"/><stop offset=".48" stop-color="${config.metal2}"/><stop offset=".72" stop-color="${config.metal1}"/><stop offset="1" stop-color="${config.metal2}"/></linearGradient>
  <radialGradient id="${ids.field}" cx="38%" cy="28%" r="78%"><stop stop-color="${config.field2}"/><stop offset=".64" stop-color="${config.field}"/><stop offset="1" stop-color="#030403"/></radialGradient>
  <radialGradient id="${ids.core}" cx="36%" cy="28%" r="75%"><stop stop-color="${config.field2}"/><stop offset="1" stop-color="${config.field}"/></radialGradient>
  <pattern id="${ids.lines}" width="13" height="13" patternUnits="userSpaceOnUse" patternTransform="rotate(24)"><path d="M0 1H13M0 7H13" stroke="${config.metal1}" stroke-width=".65" opacity=".2"/></pattern>
  ${rankShapes.map((shape, index) => `<clipPath id="${config.slug}-clip-${index + 1}">${silhouette(shape, .79)}</clipPath>`).join("\n  ")}
</defs>
<rect width="1600" height="1000" fill="#090a08"/>
<path d="M64 57H1536M64 958H1536" stroke="#30352b"/>
<text x="64" y="105" class="kicker">KINOSAIL SUPPORTER BADGE STUDY · ${config.slug.slice(0, 2)}</text>
<text x="64" y="164" class="title">${config.name}</text>
<text x="64" y="204" class="note">${config.note}. Pure SVG geometry.</text>
<text x="1536" y="164" text-anchor="end" class="direction">TEN LEVELS</text>
<g>${badges}</g>
<style>
  text{font-family:Inter,ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}.kicker{fill:${config.accent};font-size:15px;font-weight:800;letter-spacing:4px}.title{fill:#f6f8ef;font-size:50px;font-weight:790;letter-spacing:-2.4px}.note{fill:#9ca391;font-size:20px}.direction{fill:${config.metal1};font-size:16px;font-weight:800;letter-spacing:4px}.level{fill:${config.accent};font-size:11px;font-weight:800;letter-spacing:2px}.rank{fill:#f6f8ef;font-size:16px;font-weight:760;letter-spacing:.6px}
</style>
</svg>`;
}

await mkdir(outputDirectory, { recursive: true });
for (const direction of directions) {
  await writeFile(join(outputDirectory, `${direction.slug}.svg`), sheet(direction));
}
