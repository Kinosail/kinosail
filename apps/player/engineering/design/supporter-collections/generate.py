"""Regenerate all original SVG badges, contact sheets, and certificate art."""
import json
from pathlib import Path
from html import escape
from art import RANKS, SETS, badge, svg

ROOT = Path(__file__).resolve().parent
MONTHLY = [3,5,8,12,18,25,35,45,60,75]
ANNUAL = [12,40,64,96,144,200,280,360,480,600]
ONCE = [5,15,30,60,100,150,250,400,550,750]

def certificate(index, rank, family):
    title = 'Living Standard' if family == 'living' else 'Patron Order'
    record = 'A record of recurring support · Issued September 6, 2026' if family == 'living' else 'Permanent recognition · Issued September 6, 2026'
    drawing = badge(index,rank,family,prefix='certificate')
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 800" role="img" aria-labelledby="title">
<title id="title">Sample {escape(RANKS[rank-1])} {title} certificate</title>
<rect width="1200" height="800" fill="#090a08"/><rect x="28" y="28" width="1144" height="744" rx="3" fill="none" stroke="#726a4f"/>
<path d="M48 92V48H92 M1108 48H1152V92 M48 708V752H92 M1108 752H1152V708" fill="none" stroke="#e4cf9f" stroke-width="2"/>
<g font-family="Arial, sans-serif" text-anchor="middle">
<text x="600" y="96" fill="#dfe7d5" font-size="18" letter-spacing="6">KINOSAIL PLAYER</text>
<g transform="translate(453 114) scale(.82)">{drawing}</g>
<text x="600" y="457" fill="#d8c59c" font-size="14" letter-spacing="4">{title.upper()}</text>
<text x="600" y="524" fill="#f6f8ef" font-size="54">{RANKS[rank-1]}</text>
<text x="600" y="570" fill="#c0c7b6" font-size="18">With gratitude for helping Kinosail continue to grow.</text>
<text x="600" y="616" fill="#aeb6a2" font-size="14">{record}</text>
<path d="M525 656H675" stroke="#726a4f"/>
<text x="600" y="693" fill="#c6cebb" font-size="16">Thank you for being part of the story.</text>
<text x="600" y="728" fill="#aeb6a2" font-size="11" letter-spacing="2">DESIGN SAMPLE · NOT PROOF OF PAYMENT</text>
</g></svg>'''

def sheet(index):
    slug, name, note, *_ = SETS[index]
    body = '<rect width="2400" height="1100" fill="#090a08"/>'
    body += f'<g font-family="Arial, sans-serif" fill="#f6f8ef"><text x="56" y="67" font-size="34">{index+1:02d} / {escape(name)}</text><text x="56" y="105" font-size="18" fill="#aeb6a2">{escape(note)}</text>'
    for row,family in enumerate(['living','patron']):
        y = 165+row*440
        body += f'<text x="56" y="{y}" font-size="17" letter-spacing="2">{"LIVING STANDARD · RECURRING" if family == "living" else "PATRON ORDER · ONE-TIME"}</text>'
        for rank in range(1,11):
            x = 30+(rank-1)*235
            body += f'<g transform="translate({x} {y+12}) scale(.64)">{badge(index,rank,family,prefix=f"s{row}-{rank}")}</g>'
            body += f'<text x="{x+115}" y="{y+279}" text-anchor="middle" font-size="18">{rank:02d} · {RANKS[rank-1]}</text>'
            body += f'<g transform="translate({x+92} {y+304}) scale(.13)">{badge(index,rank,family,True,prefix=f"c{row}-{rank}")}</g>'
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2400 1100" role="img" aria-label="{escape(name)} complete twenty-badge contact sheet">{body}</g></svg>'

def main():
    for index,(slug,*_) in enumerate(SETS):
        folder = ROOT/'art'/slug
        folder.mkdir(parents=True,exist_ok=True)
        for family in ['living','patron']:
            for rank in range(1,11):
                for compact in [False,True]:
                    suffix = '-small' if compact else ''
                    (folder/f'{family}-{rank:02d}{suffix}.svg').write_text(svg(index,rank,family,compact))
                (folder/f'{family}-{rank:02d}-certificate.svg').write_text(certificate(index,rank,family))
        (folder/'collection.svg').write_text(sheet(index))
    catalog = dict(ranks=RANKS, sets=[dict(slug=s[0],name=s[1],note=s[2]) for s in SETS],monthly=MONTHLY,annual=ANNUAL,once=ONCE)
    (ROOT/'catalog.js').write_text('const catalog = '+json.dumps(catalog,indent=2)+';\n')
    print('Generated 200 display SVGs, 200 compact SVGs, 200 sample certificates, and 10 collection sheets.')

if __name__ == '__main__': main()
