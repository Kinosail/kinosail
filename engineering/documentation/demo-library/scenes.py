import math
import random
from html import escape


def ridge(rng, w, base, rough, color, opacity=1.0, steps=24):
    points, y = [], base
    for i in range(steps + 1):
        y += rng.uniform(-rough, rough)
        y = min(max(y, base - rough * 3), base + rough * 1.5)
        points.append(f"{w * i / steps:.0f},{y:.0f}")
    return f'<polygon points="0,{base + 4000} {" ".join(points)} {w},{base + 4000}" fill="{color}" opacity="{opacity}"/>'


def mix(a, b, t):
    pa = [int(a[i:i + 2], 16) for i in (1, 3, 5)]
    pb = [int(b[i:i + 2], 16) for i in (1, 3, 5)]
    return "#" + "".join(f"{round(x + (y - x) * t):02x}" for x, y in zip(pa, pb))


def motif(name, w, h, rng, f):
    top, low = f["sky"]
    accent, ground = f["accent"], f["ground"]
    hz = h * 0.66
    out = []
    if name == "stars":
        for _ in range(int(w * h / 7000)):
            out.append(f'<circle cx="{rng.uniform(0, w):.0f}" cy="{rng.uniform(0, hz):.0f}" r="{rng.uniform(.6, 2.2):.1f}" fill="#fff" opacity="{rng.uniform(.25, .95):.2f}"/>')
    elif name == "aurora":
        for i, color in enumerate(("#3be8b0", "#6c63ff", "#38bdf8")):
            y0 = h * (0.18 + i * 0.07)
            d = f"M0 {y0:.0f} " + " ".join(f"Q{w * (k + .5) / 4:.0f} {y0 + rng.uniform(-h * .12, h * .12):.0f} {w * (k + 1) / 4:.0f} {y0 + rng.uniform(-h * .05, h * .05):.0f}" for k in range(4))
            out.append(f'<path d="{d}" stroke="{color}" stroke-width="{h * .06:.0f}" fill="none" opacity=".35" filter="url(#blur)"/>')
    elif name in ("ridges", "snow_ridges", "hills", "paper_hills"):
        snow = name == "snow_ridges"
        for i in range(4):
            t = (i + 1) / 4
            color = mix(low, "#ffffff" if snow else ground, t * (0.55 if snow else 1))
            if name == "paper_hills":
                color = mix(low, ground, t * .8)
            rough = h * (0.03 if name in ("hills", "paper_hills") else 0.05) * (1.2 - t * .4)
            out.append(ridge(rng, w, hz - h * 0.22 + i * h * 0.075, rough, color, 1, 12 if "hills" in name else 28))
            if name == "paper_hills":
                out[-1] = out[-1].replace("/>", ' filter="url(#shadow)"/>')
    elif name == "observatory":
        x, y, r = w * .64, hz - h * .02, min(w, h) * .1
        out.append(f'<polygon points="{x - r * .3},{y - r * 1.4} {x + r * .3},{y - r * 1.4} {x + r * 2.4},{-h} {x - r * 1.2},{-h}" fill="{accent}" opacity=".16" filter="url(#blur)"/>')
        out.append(f'<rect x="{x - r}" y="{y - r * .9}" width="{2 * r}" height="{h}" fill="{ground}"/>')
        out.append(f'<path d="M{x - r} {y - r * .9} A{r} {r} 0 0 1 {x + r} {y - r * .9}Z" fill="{ground}"/>')
        out.append(f'<rect x="{x - r * .12}" y="{y - r * 1.9}" width="{r * .24}" height="{r * .9}" fill="{accent}"/>')
        out.append(ridge(rng, w, hz + h * .02, h * .015, ground, 1, 20))
    elif name == "fog":
        for _ in range(9):
            out.append(f'<ellipse cx="{rng.uniform(0, w):.0f}" cy="{rng.uniform(h * .35, h * .8):.0f}" rx="{w * rng.uniform(.3, .6):.0f}" ry="{h * .06:.0f}" fill="#ffffff" opacity=".22" filter="url(#blur)"/>')
    elif name == "sea":
        out.append(f'<rect x="0" y="{hz:.0f}" width="{w}" height="{h - hz:.0f}" fill="url(#seagrad)"/>')
        for _ in range(int(w / 9)):
            y = rng.uniform(hz, h)
            out.append(f'<rect x="{rng.uniform(-50, w):.0f}" y="{y:.0f}" width="{rng.uniform(20, w * .15):.0f}" height="1.6" fill="{accent}" opacity="{.05 + .25 * (1 - (y - hz) / (h - hz)):.2f}"/>')
    elif name == "lighthouse":
        x, y, s = w * .72, hz + h * .02, min(w, h) * .05
        out.append(f'<path d="M{x - s * 4} {y + s} Q{x} {y - s * 1.6} {x + s * 5} {y + s}Z" fill="{ground}"/>')
        out.append(f'<polygon points="{x - s * .7},{y} {x + s * .7},{y} {x + s * .45},{y - s * 6} {x - s * .45},{y - s * 6}" fill="#f4f1ea"/>')
        for k in range(3):
            yy = y - s * (1.2 + k * 1.6)
            out.append(f'<rect x="{x - s}" y="{yy:.0f}" width="{2 * s}" height="{s * .55:.0f}" fill="#b23a2f" clip-path="url(#tower)"/>')
        out.append(f'<clipPath id="tower"><polygon points="{x - s * .7},{y} {x + s * .7},{y} {x + s * .45},{y - s * 6} {x - s * .45},{y - s * 6}"/></clipPath>')
        out.append(f'<polygon points="{x},{y - s * 6.6} {-w * .2},{y - s * 9} {-w * .2},{y - s * 5}" fill="{accent}" opacity=".18" filter="url(#blur)"/>')
        out.append(f'<circle cx="{x}" cy="{y - s * 6.6}" r="{s * .6}" fill="{accent}" filter="url(#glow)"/>')
        out.append(f'<rect x="{x - s * .55}" y="{y - s * 6.6}" width="{s * 1.1}" height="{s * .5}" fill="#2b2b2b"/>')
    elif name == "rain":
        for _ in range(int(w * h / 3500)):
            x, y = rng.uniform(0, w), rng.uniform(0, h)
            out.append(f'<line x1="{x:.0f}" y1="{y:.0f}" x2="{x - 6:.0f}" y2="{y + 34:.0f}" stroke="#cfd8ff" stroke-width="1" opacity="{rng.uniform(.08, .3):.2f}"/>')
    elif name in ("city", "cranes"):
        x = 0
        while x < w:
            bw, bh = rng.uniform(w * .03, w * .08), rng.uniform(h * .08, h * (.36 if name == "city" else .12))
            out.append(f'<rect x="{x:.0f}" y="{hz - bh:.0f}" width="{bw:.0f}" height="{bh + 2:.0f}" fill="{ground}"/>')
            if name == "city":
                for wx in range(int(bw // 14)):
                    for wy in range(int(bh // 22)):
                        if rng.random() < .28:
                            color = accent if rng.random() < .4 else "#ffd9a0"
                            out.append(f'<rect x="{x + 5 + wx * 14:.0f}" y="{hz - bh + 8 + wy * 22:.0f}" width="6" height="10" fill="{color}" opacity="{rng.uniform(.5, 1):.2f}"/>')
            x += bw + rng.uniform(0, w * .01)
        if name == "cranes":
            for k in range(4):
                cx, ch = w * (.12 + k * .24), h * rng.uniform(.28, .4)
                out.append(f'<path d="M{cx} {hz} L{cx} {hz - ch} L{cx + w * .14} {hz - ch} M{cx} {hz - ch} L{cx - w * .05} {hz - ch} M{cx} {hz - ch * .2} L{cx + w * .02} {hz}" stroke="{ground}" stroke-width="{w * .006:.1f}" fill="none"/>')
                out.append(f'<circle cx="{cx}" cy="{hz - ch}" r="{w * .004:.1f}" fill="{accent}" filter="url(#glow)"/>')
    elif name == "reflection":
        for _ in range(int(w / 14)):
            x = rng.uniform(0, w)
            color = accent if rng.random() < .5 else "#ffd9a0"
            out.append(f'<rect x="{x:.0f}" y="{hz:.0f}" width="{rng.uniform(2, 7):.0f}" height="{rng.uniform(h * .05, h * .3):.0f}" fill="{color}" opacity="{rng.uniform(.1, .45):.2f}" filter="url(#blur)"/>')
    elif name == "doorway":
        dw, dh, x = w * .14, h * .34, w * .5
        y = h * .6
        out.append(f'<rect width="{w}" height="{h}" fill="{ground}" opacity=".55"/>')
        out.append(f'<polygon points="{x - dw / 2},{y} {x + dw / 2},{y} {x + dw * 2.4},{h} {x - dw * 2.4},{h}" fill="{accent}" opacity=".28" filter="url(#blur)"/>')
        out.append(f'<rect x="{x - dw / 2}" y="{y - dh}" width="{dw}" height="{dh}" fill="{accent}" filter="url(#glow)"/>')
        out.append(f'<circle cx="{x}" cy="{y - dh * .55}" r="{dw * .09}" fill="#120c08"/><path d="M{x - dw * .16} {y} L{x - dw * .12} {y - dh * .48} Q{x} {y - dh * .52} {x + dw * .12} {y - dh * .48} L{x + dw * .16} {y}Z" fill="#120c08"/>')
    elif name == "train":
        cw, ch, y = w * .105, min(w, h) * .045, hz + h * .02
        out.append(f'<g transform="rotate(-7 {w * .5:.0f} {y:.0f})"><rect x="{-w * .1:.0f}" y="{y + ch:.0f}" width="{w * 1.2:.0f}" height="{ch * .18:.0f}" fill="#2c3440"/>')
        for k in range(8):
            cx = w * .02 + k * (cw + w * .006)
            out.append(f'<rect x="{cx:.0f}" y="{y:.0f}" width="{cw:.0f}" height="{ch:.0f}" rx="{ch * .18:.0f}" fill="#1f2a36"/>')
            for j in range(5):
                out.append(f'<rect x="{cx + cw * .08 + j * cw * .18:.0f}" y="{y + ch * .22:.0f}" width="{cw * .1:.0f}" height="{ch * .3:.0f}" fill="#ffe7a8"/>')
        out.append(f'<circle cx="{w * .02 + 8 * (cw + w * .006) - w * .004:.0f}" cy="{y + ch * .5:.0f}" r="{ch * .2:.0f}" fill="{accent}" filter="url(#glow)"/></g>')
    elif name == "snowfall":
        for _ in range(int(w * h / 4000)):
            out.append(f'<circle cx="{rng.uniform(0, w):.0f}" cy="{rng.uniform(0, h):.0f}" r="{rng.uniform(.8, 3):.1f}" fill="#fff" opacity="{rng.uniform(.4, .9):.2f}"/>')
    elif name in ("sun", "sun_flat", "moon"):
        r = min(w, h) * (.16 if name == "sun" else .12)
        cy = hz - r * .15 if name == "sun" else h * .3
        fill = {"sun": "#fff2c2", "sun_flat": accent, "moon": "#f2f4ff"}[name]
        out.append(f'<circle cx="{w * .5}" cy="{cy}" r="{r}" fill="{fill}"{"" if name == "sun_flat" else " filter=\"url(#glow)\""}/>')
    elif name == "sails":
        for k, (sx, ss) in enumerate(((.24, 1), (.6, .7), (.82, .45))):
            x, s = w * sx, min(w, h) * .11 * ss
            y = hz + s * .2
            out.append(f'<path d="M{x - s} {y} L{x + s} {y} L{x + s * .7} {y + s * .3} L{x - s * .7} {y + s * .3}Z" fill="#20160f"/>')
            for m, off in enumerate((-.45, .15)):
                out.append(f'<path d="M{x + s * off} {y} L{x + s * off} {y - s * (2.2 - m * .3)} L{x + s * (off + .55)} {y - s * .2}Z" fill="#2a1d14" opacity=".92"/>')
    elif name == "paper_moons":
        for k in range(5):
            r = min(w, h) * (.28 - k * .045)
            out.append(f'<circle cx="{w * (.5 + k * .025)}" cy="{h * .32}" r="{r}" fill="{mix("#fff6ee", "#ffc4b0", k / 5)}" filter="url(#shadow)"/>')
    elif name == "embers":
        for _ in range(int(w * h / 9000)):
            out.append(f'<circle cx="{rng.uniform(0, w):.0f}" cy="{rng.uniform(h * .1, h):.0f}" r="{rng.uniform(1, 4):.1f}" fill="{accent}" opacity="{rng.uniform(.3, .95):.2f}" filter="url(#glow)"/>')
    elif name == "fire_glow":
        out.append(f'<ellipse cx="{w * .5}" cy="{hz - h * .1}" rx="{w * .9}" ry="{h * .35}" fill="url(#fireglow)"/>')
    elif name == "crew":
        for k in range(4):
            x, y, s = w * (.38 + k * .06), hz + h * .06, h * .03
            out.append(f'<circle cx="{x}" cy="{y - s * 2.3}" r="{s * .42}" fill="#000"/><path d="M{x - s * .6} {y} L{x - s * .45} {y - s * 1.9} L{x + s * .45} {y - s * 1.9} L{x + s * .6} {y}Z" fill="#000"/>')
    elif name == "greenhouse":
        x0, x1, y = w * .14, w * .86, hz + h * .08
        top = y - h * .42
        out.append(f'<path d="M{x0} {y} L{x0} {top + h * .12} Q{w / 2} {top - h * .1} {x1} {top + h * .12} L{x1} {y}Z" fill="{accent}" opacity=".08" stroke="{accent}" stroke-width="3" stroke-opacity=".7"/>')
        for k in range(1, 10):
            x = x0 + (x1 - x0) * k / 10
            out.append(f'<line x1="{x:.0f}" y1="{y:.0f}" x2="{x:.0f}" y2="{top + h * .12 - math.sin(math.pi * k / 10) * h * .16:.0f}" stroke="{accent}" stroke-opacity=".45" stroke-width="2"/>')
        out.append(f'<rect x="0" y="{y:.0f}" width="{w}" height="{h}" fill="{ground}"/>')
    elif name == "leaves":
        for _ in range(14):
            x, y, s = rng.uniform(-w * .05, w * 1.05), rng.uniform(h * .65, h * 1.05), rng.uniform(.06, .14) * w
            a = rng.uniform(-60, 60)
            out.append(f'<path transform="translate({x:.0f} {y:.0f}) rotate({a:.0f})" d="M0 0 Q{s * .5} {-s * .6} 0 {-s * 1.6} Q{-s * .5} {-s * .6} 0 0Z" fill="{mix(ground, "#2f6b4a", rng.random() * .6)}"/>')
    elif name == "fireflies":
        for _ in range(40):
            out.append(f'<circle cx="{rng.uniform(w * .1, w * .9):.0f}" cy="{rng.uniform(h * .3, h * .8):.0f}" r="{rng.uniform(1.5, 3.5):.1f}" fill="{accent}" filter="url(#glow)"/>')
    elif name == "beach":
        out.append(f'<rect x="0" y="{hz:.0f}" width="{w}" height="{h * .08:.0f}" fill="#4fb3c8"/><rect x="0" y="{hz + h * .08:.0f}" width="{w}" height="{h}" fill="{ground}"/>')
    elif name == "hotel_sign":
        x, y, s = w * .5, hz - h * .05, min(w, h) * .13
        out.append(f'<rect x="{x - s * .04}" y="{y}" width="{s * .08}" height="{h * .2}" fill="#3b3b3b"/>')
        out.append(f'<rect x="{x - s * 1.4}" y="{y - s * 1.1}" width="{s * 2.8}" height="{s * 1.1}" rx="{s * .25}" fill="{accent}" stroke="#fff" stroke-width="{s * .05}"/>')
        out.append(f'<text x="{x}" y="{y - s * .38}" text-anchor="middle" font-family="Futura" font-weight="700" font-size="{s * .5:.0f}" fill="#fff" letter-spacing="{s * .08:.0f}">HOTEL</text>')
    elif name == "static":
        out.append(f'<rect width="{w}" height="{h}" filter="url(#noise)" opacity=".45"/>')
        for y in range(0, int(h), 6):
            out.append(f'<rect y="{y}" width="{w}" height="2" fill="#000" opacity=".35"/>')
    elif name == "radio_tower":
        x, y, s = w * .5, hz + h * .1, h * .5
        out.append(f'<path d="M{x - s * .12} {y} L{x} {y - s} L{x + s * .12} {y} M{x - s * .09} {y - s * .25} L{x + s * .09} {y - s * .25} M{x - s * .06} {y - s * .5} L{x + s * .06} {y - s * .5} M{x - s * .12} {y} L{x + s * .06} {y - s * .5} M{x + s * .12} {y} L{x - s * .06} {y - s * .5}" stroke="#0a140d" stroke-width="{s * .012:.1f}" fill="none"/>')
        out.append(f'<circle cx="{x}" cy="{y - s}" r="{s * .015:.1f}" fill="#ff3b3b" filter="url(#glow)"/>')
    elif name == "orchard":
        for row in range(5):
            y = hz + row * h * .07
            r = h * (.025 + row * .012)
            for k in range(int(w / (r * 3)) + 2):
                out.append(f'<circle cx="{k * r * 3 + (row % 2) * r * 1.5:.0f}" cy="{y:.0f}" r="{r:.0f}" fill="{mix(ground, "#1b2a55", .4 - row * .08)}"/><rect x="{k * r * 3 + (row % 2) * r * 1.5 - r * .1:.0f}" y="{y:.0f}" width="{r * .2:.0f}" height="{r * 1.2:.0f}" fill="{ground}"/>')
    elif name == "birds":
        for _ in range(9):
            x, y, s = rng.uniform(w * .2, w * .8), rng.uniform(h * .1, h * .4), rng.uniform(8, 18)
            out.append(f'<path d="M{x - s} {y} Q{x - s / 2} {y - s / 2} {x} {y} Q{x + s / 2} {y - s / 2} {x + s} {y}" stroke="#2c3e2c" stroke-width="2.4" fill="none"/>')
    return "".join(out)


def defs(f):
    top, low = f["sky"]
    return (f'<defs><linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{top}"/><stop offset="1" stop-color="{low}"/></linearGradient>'
            f'<linearGradient id="seagrad" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{mix(low, f["ground"], .4)}"/><stop offset="1" stop-color="{f["ground"]}"/></linearGradient>'
            f'<radialGradient id="fireglow"><stop offset="0" stop-color="{f["accent"]}" stop-opacity=".85"/><stop offset="1" stop-color="{f["accent"]}" stop-opacity="0"/></radialGradient>'
            '<radialGradient id="vignette" r=".75"><stop offset=".55" stop-color="#000" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".55"/></radialGradient>'
            '<filter id="blur" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="18"/></filter>'
            '<filter id="glow" x="-200%" y="-200%" width="500%" height="500%"><feGaussianBlur stdDeviation="6" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'
            '<filter id="shadow" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="8" stdDeviation="10" flood-opacity=".22"/></filter>'
            '<filter id="noise"><feTurbulence type="fractalNoise" baseFrequency=".9" numOctaves="2" seed="7"/><feColorMatrix values="0 0 0 0 .45  0 0 0 0 1  0 0 0 0 .6  0 0 0 1.4 -.3"/></filter>'
            '<filter id="grain"><feTurbulence type="fractalNoise" baseFrequency=".75" numOctaves="2" stitchTiles="stitch"/><feColorMatrix type="saturate" values="0"/></filter></defs>')


def scene(f, w, h, seed):
    rng = random.Random(f"{f['title']}-{seed}")
    body = "".join(motif(m, w, h, rng, f) for m in f["motifs"])
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">{defs(f)}'
            f'<rect width="{w}" height="{h}" fill="url(#sky)"/>{body}'
            f'<rect width="{w}" height="{h}" filter="url(#grain)" opacity=".07"/><rect width="{w}" height="{h}" fill="url(#vignette)"/>')


def poster(f, w=1000, h=1500, kind="Film"):
    svg = scene(f, w, h, "poster")
    lines = f["title_lines"]
    longest = max(len(line) for line in lines)
    size = min(150, int(w * 1.55 / max(longest, 4)))
    color = "#ffffff" if f["accent"] in ("#ffffff", "#fff1d6", "#e9edff") or kind == "Show" else f["accent"]
    if f.get("sky")[0].startswith("#c") or f.get("sky")[0].startswith("#f") or f.get("sky")[0].startswith("#9") or f.get("sky")[0].startswith("#7"):
        color = "#14202b" if f["title"] != "Low Tide Hotel" else "#ffffff"
    y0 = h * .14 if f.get("layout", "top") == "top" else h * .72
    text = "".join(f'<text x="{w / 2}" y="{y0 + i * size * 1.02:.0f}" text-anchor="middle" font-family="{escape(f["font"])}" font-size="{size}" font-weight="700" fill="{color}" letter-spacing="{size * .04:.0f}" filter="url(#shadow)" data-fit="{i}" data-y0="{y0:.0f}">{escape(line)}</text>' for i, line in enumerate(lines))
    tag = f.get("tagline", "")
    tag_y = y0 - size * .95 if f.get("layout", "top") == "top" else y0 + len(lines) * size * 1.02 - size * .1
    if f.get("layout", "top") == "top":
        tag_y = y0 + len(lines) * size * 1.02 - size * .2
    text += f'<text x="{w / 2}" y="{tag_y:.0f}" text-anchor="middle" font-family="Avenir Next" font-size="{w * .03:.0f}" fill="{color}" opacity=".85" letter-spacing="2" data-tag="{f.get('layout', 'top')}">{escape(tag)}</text>'
    credit = f"{f['studio'].upper()} PRESENTS" + (f" · A FILM BY {f['director'].upper()}" if kind == "Film" else " · AN ORIGINAL SERIES")
    text += f'<text x="{w / 2}" y="{h - 70}" text-anchor="middle" font-family="Avenir Next Condensed" font-size="{w * .022:.0f}" fill="{color}" opacity=".7" letter-spacing="3">{escape(credit)}</text>'
    text += f'<text x="{w / 2}" y="{h - 38}" text-anchor="middle" font-family="Avenir Next Condensed" font-size="{w * .018:.0f}" fill="{color}" opacity=".5" letter-spacing="4">FICTIONAL DEMO TITLE · {f["year"]}</text>'
    return svg + text + "</svg>"


def backdrop(f, w=1920, h=1080, seed="backdrop"):
    return scene(f, w, h, seed) + "</svg>"
