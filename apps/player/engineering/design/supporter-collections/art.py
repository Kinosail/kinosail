"""Original Kinosail vector artwork. Geometry only; no bitmap or remote assets."""
import math

RANKS = ['Friend', 'Crew', 'Navigator', 'Patron', 'Steward', 'Lighthouse', 'Commodore', 'Admiral', 'North Star', 'Legacy']
SETS = [
    ('sailcraft', 'Sailcraft', 'A little sail becomes a flagship under a celestial canopy.', '#c8dfbd', '#214039', 'sail'),
    ('beacon', 'Guiding Light', 'A harbor light grows into a monumental beacon of welcome.', '#eddba8', '#343023', 'beacon'),
    ('celestial', 'Celestial Navigation', 'One guiding star becomes an intricate astronomical instrument.', '#bcd3e9', '#212d40', 'star'),
    ('orders', 'Maritime Orders', 'A sailor’s knot develops into a finely engraved ceremonial order.', '#e4c58d', '#34302a', 'knot'),
    ('cinema', 'Cinema Editions', 'A frame of light becomes an architectural tribute to cinema.', '#e4b6a2', '#402d2a', 'cinema'),
    ('chartmaker', 'Chartmaker', 'A bearing grows into a complete instrument for finding your way.', '#d9d6ad', '#30372c', 'compass'),
    ('harbor', 'Harbor Architecture', 'An open doorway develops into a welcoming waterfront landmark.', '#b9d4c9', '#243a36', 'harbor'),
    ('whales', 'Ocean Giants', 'A quiet whale becomes a magnificent guardian of the open ocean.', '#a8d5df', '#183943', 'whale'),
    ('monogram', 'Sail & Signal', 'Two simple strokes become an interwoven Kinosail signature.', '#d3dbb7', '#303925', 'monogram'),
    ('heirloom', 'Legacy Seals', 'A maker’s mark grows into an heirloom of leaves, light, and sea.', '#e8cc9c', '#3a3024', 'seal'),
]

def path(d, fill='none', stroke='currentColor', width=2, extra=''):
    return f'<path d="{d}" fill="{fill}" stroke="{stroke}" stroke-width="{width}" stroke-linecap="round" stroke-linejoin="round" {extra}/>'

def circle(x, y, r, fill='none', stroke='currentColor', width=2):
    return f'<circle cx="{x}" cy="{y}" r="{r}" fill="{fill}" stroke="{stroke}" stroke-width="{width}"/>'

def star(x, y, r, points=4, inner=.3, fill='currentColor'):
    coords = []
    for i in range(points*2):
        a = -math.pi/2 + i*math.pi/points
        rad = r if i % 2 == 0 else r*inner
        coords.append(f'{x+math.cos(a)*rad:.2f},{y+math.sin(a)*rad:.2f}')
    return f'<polygon points="{" ".join(coords)}" fill="{fill}"/>'

def wave(y, width=56):
    return path(f'M{-width} {y} Q{-width/2} {y-12} 0 {y} T{width} {y}', width=2.4)

def emblem(kind, rank, compact=False):
    """Each subject grows at deliberate thresholds, independently of its frame."""
    r = rank
    out = ''
    if kind == 'sail':
        out = path('M-6 -55 L-6 31 L-54 31 Z', 'currentColor', 'none')
        out += path('M3 -39 Q34 -16 44 27 H3 Z', 'currentColor', 'none')
        out += path('M-58 40 H56 L34 53 H-34 Z', 'currentColor', 'none')
        if r >= 3: out += path('M7 -57 H30 L17 -47 H7', 'currentColor', 'none')
        if r >= 5: out += path('M-60 12 L-28 -37 M48 15 L24 -28', width=2)
        if r >= 7: out += wave(65, 67)
        if r >= 9: out += star(0, -78, 12, 4)
        if r == 10: out += star(-49, -52, 7) + star(49, -52, 7)
    elif kind == 'beacon':
        out = path('M-24 50 L-13 -30 H13 L24 50 Z', 'currentColor', 'none')
        out += path('M-19 -35 V-51 H19 V-35 M-25 -55 L0 -72 L25 -55 M-31 56 H31', width=4)
        if r >= 2: out += path('M-33 -40 H-49 M33 -40 H49', width=3)
        if r >= 4: out += path('M-30 -52 L-61 -68 M30 -52 L61 -68', width=3)
        if r >= 6: out += path('M-55 53 Q-44 23 -28 34 M55 53 Q44 23 28 34', width=3)
        if r >= 7: out += wave(70, 66)
        if r >= 8: out += path('M-35 -23 L-66 -8 M35 -23 L66 -8', width=2)
        if r == 10: out += star(0, -92, 11) + circle(0, -43, 4, 'currentColor')
    elif kind == 'star':
        out = star(0, 0, 61, 4, .24)
        if r >= 2: out += circle(0, 0, 30, width=1.6)
        if r >= 3: out += f'<g transform="rotate(45)">{star(0,0,46,4,.15)}</g>'
        if r >= 4: out += circle(0, 0, 68, width=1.6)
        if r >= 6: out += '<ellipse rx="83" ry="34" transform="rotate(-30)" fill="none" stroke="currentColor" stroke-width="2"/>'
        if r >= 7: out += circle(-71, 39, 5, 'currentColor') + circle(70, -40, 4, 'currentColor')
        if r >= 9: out += star(0,-82,9) + star(0,82,9)
        if r == 10: out += '<ellipse rx="83" ry="34" transform="rotate(30)" fill="none" stroke="currentColor" stroke-width="1.5"/>'
    elif kind == 'knot':
        out = path('M-61 4 C-77 -26 -43 -58 -17 -31 L27 24 C53 54 80 21 61 -5 C43 -33 22 -16 0 11 L-23 36 C-47 63 -78 29 -61 4 Z', width=9 if compact else 7)
        out += path('M-12 -23 L17 14',stroke='#15211b',width=13 if compact else 11)
        out += path('M-12 -23 L17 14',width=9 if compact else 7)
        if r >= 3: out += path('M-32 -22 L-52 -55 M33 20 L53 53', width=5)
        if r >= 5: out += circle(0,0,74,width=1.4)
        if r >= 7: out += star(0,-82,12,4)
        if r >= 9: out += path('M-71 48 L-43 80 L0 65 L43 80 L71 48', width=3)
        if r == 10: out += star(-75,-40,9) + star(75,-40,9)
    elif kind == 'cinema':
        out = path('M-54 -37 H54 V37 H-54 Z', width=5)
        out += path('M-12 -22 L25 0 L-12 22 Z', 'currentColor', 'none')
        if r >= 2: out += path('M-62 47 H62',width=3)
        if r >= 4: out += path('M-65 -49 H65 V47 M-65 -49 V47',width=2)
        if r >= 5: out += path('M-75 -57 H75 M-75 -57 V56 M75 -57 V56',width=3)
        if r >= 7: out += path('M-75 -65 L0 -88 L75 -65 M-49 -65 L0 -80 L49 -65',width=2)
        if r >= 8: out += path('M-68 60 H68 M-58 69 H58',width=3)
        if r == 10: out += star(0,-67,9) + star(-40,63,5) + star(40,63,5)
    elif kind == 'compass':
        out = path('M-17 23 L0 -60 L17 -23 L0 60 Z','currentColor','none')
        out += circle(0,0,44,width=2)
        if r >= 2: out += circle(0,0,6, '#090a08')
        if r >= 3: out += path('M-61 0 H-49 M49 0 H61',width=4)
        if r >= 4: out += circle(0,0,65,width=1.5)
        if r >= 6: out += path('M-47 -47 L-38 -38 M47 -47 L38 -38 M-47 47 L-38 38 M47 47 L38 38',width=3)
        if r >= 7: out += path('M-77 0 H-67 M77 0 H67 M0 -77 V-67 M0 77 V67',width=3)
        if r >= 9: out += circle(0,0,81,width=1)
        if r == 10: out += star(-57,-57,7)+star(57,57,7)+star(-57,57,7)+star(57,-57,7)
    elif kind == 'harbor':
        out = path('M-35 51 V-15 A35 35 0 0 1 35 -15 V51 M-43 51 H43',width=6)
        out += path('M-19 46 V-12 A19 19 0 0 1 19 -12 V46',width=2)
        if r >= 3: out += path('M-49 51 V-20 A49 49 0 0 1 49 -20 V51',width=2)
        if r >= 5: out += path('M-71 51 V-14 H-55 M71 51 V-14 H55',width=4)
        if r >= 7: out += path('M-77 -25 H-54 M77 -25 H54 M-65 -25 V-51 M65 -25 V-51',width=3)
        if r >= 8: out += wave(65,77)
        if r >= 9: out += path('M-31 -66 L0 -82 L31 -66',width=3)
        if r == 10: out += star(0,-65,8) + path('M-62 80 H62',width=2)
    elif kind == 'whale':
        out = path('M-70 4 C-67 -20 -30 -30 -4 -13 C19 5 39 11 49 -13 C42 -31 52 -43 71 -45 C74 -25 68 -14 58 -7 C67 -5 78 4 80 18 C61 24 50 14 46 2 C35 50 -34 60 -64 26 Z','currentColor','none')
        out += circle(-49,5,3,'#090a08','none')
        out += path('M-8 26 Q-2 50 -25 55 Q-21 32 -8 26','#090a08','none')
        if r >= 2: out += path('M-36 -35 Q-35 -49 -45 -50 M-35 -35 Q-30 -53 -21 -50',width=3)
        if r >= 4: out += wave(66,67)
        if r >= 6: out += path('M-66 33 Q-12 72 36 36',width=2)
        if r >= 7: out += star(0,-71,13)
        if r >= 8: out += star(-37,-65,7) + star(36,-63,7)
        if r >= 9: out += wave(80,49)
        if r == 10: out += path('M-69 -45 Q0 -109 69 -45',width=1.5)+circle(-63,-49,3,'currentColor')+circle(63,-49,3,'currentColor')
    elif kind == 'monogram':
        out = path('M-34 -56 V56 M34 -56 L-21 0 L40 56',width=12 if compact else 9)
        if r >= 2: out += path('M-43 65 H48',width=3)
        if r >= 4: out += path('M-19 -55 V-13 M-19 21 V55 M39 -43 L1 -3 L49 42',width=2)
        if r >= 6: out += path('M-58 -57 V57 M58 -57 V17',width=2)
        if r >= 7: out += path('M-48 -68 H47 M-48 77 H47',width=3)
        if r >= 9: out += star(0,-78,10)
        if r == 10: out += path('M-69 -40 V40 M69 -40 V40',width=4)
    else:
        out = path('M0 -57 Q-43 -10 -27 22 Q-11 47 0 60 Q11 47 27 22 Q43 -10 0 -57 Z',width=3)
        out += path('M0 -40 V45 M0 9 L-20 -10 M0 27 L20 4',width=4)
        if r >= 2: out += circle(0,0,63,width=1.5)
        if r >= 4: out += path('M-15 -56 L0 -72 L15 -56',width=3)
        if r >= 6: out += wave(74,48)
        if r >= 7: out += star(-64,-39,8)+star(64,-39,8)
        if r >= 9: out += path('M-67 23 Q-69 61 -36 72 M67 23 Q69 61 36 72',width=3)
        if r == 10: out += star(0,-88,10)+circle(0,86,5,'currentColor')
    return out

def border(rank, living, fill, stroke, compact, kind):
    # A circle, cut coin, shield, order, and finally a sculpted star.
    if kind in ['cinema','harbor','monogram','whale','beacon','star','compass']:
        shapes = {
            'cinema': 'M-94 -108 H94 V-90 H110 V94 H94 V110 H-94 V94 H-110 V-90 H-94 Z',
            'harbor': 'M-104 111 V-20 A104 104 0 0 1 104 -20 V111 Z',
            'monogram': 'M0 -127 L109 -42 V42 L0 127 L-109 42 V-42 Z',
            'whale': 'M0 -103 C79 -103 119 -62 119 0 C119 66 66 107 0 107 C-66 107 -119 66 -119 0 C-119 -62 -79 -103 0 -103 Z',
            'beacon': 'M0 -124 L87 -89 L105 48 L77 109 H-77 L-105 48 L-87 -89 Z',
            'star': 'M0 -123 L24 -97 L86 -86 L97 -24 L123 0 L97 24 L86 86 L24 97 L0 123 L-24 97 L-86 86 L-97 24 L-123 0 L-97 -24 L-86 -86 L-24 -97 Z',
            'compass': 'M0 -121 L34 -88 L85 -85 L88 -34 L121 0 L88 34 L85 85 L34 88 L0 121 L-34 88 L-85 85 L-88 34 L-121 0 L-88 -34 L-85 -85 L-34 -88 Z',
        }
        s = path(shapes[kind],fill,stroke,4 if compact else 2.5)
        if living:
            s += path('M-70 99 V127 L-32 114 L0 140 L32 114 L70 127 V99',fill,stroke,2.5)
        if rank >= 5:
            s += f'<g transform="scale(1.09)">{path(shapes[kind],"none",stroke,1.4)}</g>'
        return s
    if living:
        d = f'M-98 -100 Q0 {-125-rank} 98 -100 V65 L{55+rank} 54 L0 {109+rank} L{-55-rank} 54 L-98 65 Z'
        return path(d,fill,stroke,4 if compact else 2.5)
    if rank < 3: return circle(0,0,103,fill,stroke,4 if compact else 2.5)
    if rank < 5:
        return path('M-72 -92 H72 L105 -42 V42 L72 92 H-72 L-105 42 V-42 Z',fill,stroke,3)
    if rank < 7:
        return path('M0 -111 L101 -71 V29 Q101 87 0 115 Q-101 87 -101 29 V-71 Z',fill,stroke,3)
    n = 8 if rank == 7 else 12 if rank == 8 else 16 if rank == 9 else 20
    points = []
    for i in range(n*2):
        a = -math.pi/2+i*math.pi/n
        rad = (117 if rank < 9 else 124) if i%2 == 0 else 106
        points.append(f'{math.cos(a)*rad:.2f},{math.sin(a)*rad:.2f}')
    return f'<polygon points="{" ".join(points)}" fill="{fill}" stroke="{stroke}" stroke-width="3" stroke-linejoin="round"/>'

def ornaments(rank, living, color, compact, kind):
    s = ''
    if rank >= 2:
        s += circle(0,0,94,stroke=color,width=1.5 if compact else .9)
    if rank >= 3 and not compact:
        for i in range(rank*4):
            a = i*360/(rank*4)
            s += f'<g transform="rotate({a})">{path("M0 -88 V-91",stroke=color,width=1)}</g>'
    if rank >= 3:
        s += path('M-23 -99 L0 -111 L23 -99',stroke=color,width=3)
    if rank >= 4:
        for x in [-1,1]:
            s += f'<g transform="translate({x*110} 0)">{star(0,0,5+rank/2,4,fill=color)}</g>'
    if rank >= 5 and kind in ['sail','knot','seal']:
        s += path('M-49 112 L-58 145 L-33 132 L-16 149 L-9 117 M49 112 L58 145 L33 132 L16 149 L9 117',color,'none')
    if rank >= 6 and kind in ['sail','knot','seal']:
        s += path('M-64 -102 L-42 -130 H42 L64 -102',stroke=color,width=3)
    if rank >= 7 and kind in ['sail','knot','seal']:
        for side in [-1,1]:
            s += f'<g transform="scale({side} 1)">' + path('M46 120 Q149 77 128 -32',stroke=color,width=2.5)
            for i in range(3 if compact else rank-1):
                y = 92-i*(22 if compact else 18)
                x = 96+22*math.sin((i+1)*.45)
                s += path(f'M{x-3} {y+12} Q{x+26} {y+6} {x+20} {y-17} Q{x} {y-11} {x-3} {y+12}',color,'none')
            s += '</g>'
    if rank >= 6 and kind in ['cinema','harbor','monogram']:
        for side in [-1,1]:
            s += f'<g transform="scale({side} 1)">' + path(f'M120 103 V-86 L{129+rank} -103 V114 H108',stroke=color,width=3) + '</g>'
        if rank >= 8: s += path('M-91 -122 L0 -158 L91 -122 M-88 137 H88',stroke=color,width=3)
        if rank == 10: s += path('M-77 -134 L0 -175 L77 -134 M-77 150 H77 M-65 160 H65',stroke=color,width=2)
    if rank >= 6 and kind in ['star','compass','beacon']:
        for i in range(4 if rank < 8 else 8):
            angle = i * (90 if rank < 8 else 45)
            s += f'<g transform="rotate({angle})">{path(f"M0 -130 V{-135-rank*3}",stroke=color,width=3)}</g>'
        if rank >= 8: s += circle(0,0,146,stroke=color,width=1)
        if rank == 10:
            for angle in [45,135,225,315]:
                s += f'<g transform="rotate({angle})">{star(0,-157,9,4,fill=color)}</g>'
    if rank >= 6 and kind == 'whale':
        for side in [-1,1]:
            s += f'<g transform="scale({side} 1)">' + path('M37 128 C136 110 165 7 122 -53 C149 34 86 49 111 87 C96 110 65 119 37 128',fill=color,stroke='none') + '</g>'
        if rank >= 8: s += path('M-89 -108 Q0 -164 89 -108',stroke=color,width=2)
        if rank >= 9: s += star(0,-145,15,4,fill=color)
        if rank == 10: s += path('M-61 140 Q0 162 61 140 M-41 155 Q0 168 41 155',stroke=color,width=3)+star(-48,-132,8,4,fill=color)+star(48,-132,8,4,fill=color)
    if rank >= 8 and kind in ['sail','knot','seal']:
        s += star(0,-135,19 if rank == 8 else 23,4,fill=color)
    if rank >= 9 and kind in ['sail','knot','seal']:
        s += path('M-78 -104 L-59 -149 L-31 -130 L0 -161 L31 -130 L59 -149 L78 -104',stroke=color,width=3)
    if rank == 10 and kind in ['sail','knot','seal']:
        s += star(0,-168,12,4,fill=color)
        s += path('M-80 125 Q0 170 80 125 L70 147 Q0 184 -70 147 Z',fill=color,stroke='none')
        if not compact:
            for x in [-48,-24,0,24,48]: s += star(x,151-abs(x)*.16,3,4,fill='#202719')
    if living:
        s += path('M-75 -105 Q0 -129 75 -105',stroke=color,width=5)
        s += path('M-15 111 L0 128 L15 111',stroke=color,width=3)
    else:
        s += circle(0,104,4 if rank<6 else 6,color,'none')
    return s

def badge(index, rank, family, compact=False, prefix='b'):
    slug, name, _, accent, field, kind = SETS[index]
    living = family == 'living'
    metal = '#c9edce' if living else '#eed29b'
    low = '#739f90' if living else '#947347'
    if rank <= 3: metal = accent
    defs = f'<defs><linearGradient id="{prefix}-metal" x1="0" y1="0" x2="1" y2="1"><stop stop-color="{metal}"/><stop offset=".35" stop-color="#fff4dc"/><stop offset=".6" stop-color="{low}"/><stop offset=".83" stop-color="{metal}"/></linearGradient><linearGradient id="{prefix}-field" x2="0" y2="1"><stop stop-color="{field}"/><stop offset="1" stop-color="#0c1310"/></linearGradient></defs>'
    ink = metal if compact else f'url(#{prefix}-metal)'
    shell = border(rank,living,f'url(#{prefix}-field)',ink,compact,kind)
    decor = ornaments(rank,living,ink,compact,kind)
    motif = f'<g color="{accent}" transform="scale({.83 if compact else .87})">{emblem(kind,rank,compact)}</g>'
    return defs + f'<g transform="translate(180 180)">{shell}{decor}{motif}</g>'

def svg(index, rank, family, compact=False):
    title = f'{SETS[index][1]} — {RANKS[rank-1]} — {"Living Standard" if family == "living" else "Patron Order"}'
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 360 360" role="img" aria-labelledby="title"><title id="title">{title.replace("&", "&amp;")}</title>{badge(index,rank,family,compact)}</svg>'
