"""Validate export integrity, vector-only content, and complete badge families."""
import re
import xml.etree.ElementTree as ET
from pathlib import Path
from art import SETS

ROOT = Path(__file__).resolve().parent
NS = '{http://www.w3.org/2000/svg}'
all_badges = []
compact_badges = []
for slug,*_ in SETS:
    for family in ['living','patron']:
        previous = None
        for rank in range(1,11):
            for suffix in ['', '-small', '-certificate']:
                file = ROOT/'art'/slug/f'{family}-{rank:02d}{suffix}.svg'
                source = file.read_text()
                root = ET.fromstring(source)
                assert root.tag == NS+'svg', file
                assert not re.search(r'<(?:image|script|foreignObject)\b|data:|https?://(?!www.w3.org)',source), file
                ids = [element.attrib['id'] for element in root.iter() if 'id' in element.attrib]
                assert len(ids) == len(set(ids)), file
                for reference in re.findall(r'url\(#([^)]*)\)',source): assert reference in ids, (file,reference)
                assert '<title' in source, file
                if suffix == '':
                    # Compare actual paths/shapes, not rank names in accessibility labels.
                    geometry = source[source.index('<defs>'):]
                    assert geometry != previous, file
                    previous = geometry
                    all_badges.append(geometry)
                if suffix == '-small': compact_badges.append(source[source.index('<defs>'):])
                if suffix == '-certificate':
                    assert 'NOT PROOF OF PAYMENT' in source, file
                    assert not re.search(r'\$|installationKey|supporterId',source), file
                    if family == 'living': assert 'record of recurring support' in source, file
                    else: assert 'Permanent recognition' in source, file
    ET.parse(ROOT/'art'/slug/'collection.svg')
assert len(all_badges) == 200
assert len(set(all_badges)) == 200
assert len(set(compact_badges)) == 200
print('PASS: 200 unique display and 200 unique compact artworks. 600 badge/certificate SVGs pass vector, paint-reference, and label checks; 10 collection sheets parse.')
print('PASS: certificate samples omit amounts and private identifiers; both independent families and all 10 ranks exist.')
