"""Portable review booklet; Cairo keeps SVG paths as vector PDF graphics."""
import io
from html import escape
from pathlib import Path
import zipfile
import cairosvg
from pypdf import PdfReader, PdfWriter
from art import RANKS, SETS, badge

ROOT = Path(__file__).resolve().parent
OUT = ROOT/'output'/'pdf'
PREVIEWS = ROOT/'previews'
OUT.mkdir(parents=True,exist_ok=True)
PREVIEWS.mkdir(exist_ok=True)

def canvas(width, height, content):
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}"><rect width="{width}" height="{height}" fill="#090a08"/>{content}</svg>'

def text(x,y,value,size=20,fill='#f6f8ef',anchor='start'):
    return f'<text x="{x}" y="{y}" font-family="Arial,sans-serif" font-size="{size}" fill="{fill}" text-anchor="{anchor}">{escape(value)}</text>'

def overview(start):
    body = text(36,50,'KINOSAIL / SUPPORTER COLLECTIONS',18,'#dec99e')
    body += text(36,95,f'Ten ways to say thank you. / {start//5+1} of 2',30)
    body += text(36,128,'Friend → Lighthouse → Legacy · One-time family shown',17,'#afb8a3')
    for row,index in enumerate(range(start,start+5)):
        y = 175+row*300
        body += text(36,y,f'{index+1:02d}  {SETS[index][1]}',24)
        for col,rank in enumerate([1,6,10]):
            x = 25+col*245
            body += f'<g transform="translate({x} {y+10}) scale(.62)">{badge(index,rank,"patron",prefix=f"o{row}-{rank}")}</g>'
            body += text(x+112,y+250,RANKS[rank-1],16,'#afb8a3','middle')
    body += text(36,1700,'Original SVG artwork · Two families · Ten ranks per collection',16,'#afb8a3')
    return canvas(780,1735,body)

def page(index, start):
    body = text(40,48,'KINOSAIL PLAYER / VECTOR ART EXPLORATION',16,'#dec99e')
    body += text(40,97,f'{index+1:02d}  {SETS[index][1]}',34)
    body += text(40,132,f'Ranks {start}–{start+4} · '+('A beautiful beginning.' if start==1 else 'An exceptional contribution.'),18,'#afb8a3')
    body += text(236,175,'LIVING STANDARD',16,'#c9edce','middle')
    body += text(650,175,'PATRON ORDER',16,'#eed29b','middle')
    for row,rank in enumerate(range(start,start+5)):
        y = 193+row*252
        for col,family in enumerate(['living','patron']):
            x = 126+col*414
            body += f'<g transform="translate({x} {y}) scale(.6)">{badge(index,rank,family,prefix=f"p{rank}-{col}")}</g>'
            body += text(x+108,y+239,f'{rank:02d} · {RANKS[rank-1]}',20,'#f6f8ef','middle')
    body += text(40,1494,f'{index*2+(1 if start==1 else 2):02d} / 20   •   Original editable SVGs included separately.',15,'#afb8a3')
    return canvas(900,1530,body)

writer = PdfWriter()
for index in range(10):
    for start in [1,6]:
        source = page(index,start)
        pdf = cairosvg.svg2pdf(bytestring=source.encode())
        writer.add_page(PdfReader(io.BytesIO(pdf)).pages[0])
        # All review pages also remain available as independent vector sheets.
        (PREVIEWS/f'{index+1:02d}-{start:02d}.svg').write_text(source)
for start in [0,5]:
    source = overview(start)
    filename = PREVIEWS/f'collections-{start//5+1}'
    filename.with_suffix('.svg').write_text(source)
    cairosvg.svg2png(bytestring=source.encode(),write_to=str(filename.with_suffix('.png')))
writer.add_metadata({'/Title':'Kinosail - Ten Supporter Collections','/Author':'Kinosail','/Subject':'200 original vector badge concepts; design samples'})
with (OUT/'kinosail-supporter-collections.pdf').open('wb') as output: writer.write(output)
for start in [1,6]:
    source = page(0,start)
    cairosvg.svg2png(bytestring=source.encode(),write_to=str(PREVIEWS/f'sailcraft-{start:02d}.png'))
with zipfile.ZipFile(ROOT/'supporter-art.zip','w',zipfile.ZIP_DEFLATED) as bundle:
    for folder in [ROOT/'art',PREVIEWS,OUT]:
        for file in sorted(folder.rglob('*')):
            if file.is_file(): bundle.write(file,file.relative_to(ROOT))
    for name in ['index.html','supporter.html','catalog.js','studio.css','studio.js','supporter-preview.js','README.md','art.py','generate.py','export_review.py','verify.py']:
        if (ROOT/name).exists(): bundle.write(ROOT/name,name)
print('Exported 20-page vector PDF, two phone overview images, and complete SVG archive.')
