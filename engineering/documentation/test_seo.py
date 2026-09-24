import json
import unittest
from seo_check import validate


class SearchMetadataTest(unittest.TestCase):
    def fixture(self, url='https://example.org/kinosail/'):
        data = {'@context': 'https://schema.org', '@graph': [{'@type': 'WebPage', 'url': url}]}
        image = 'https://example.org/assets/share.png'
        return (f'<link rel="canonical" href="{url}"><meta name="description" content="Media library">'
                f'<meta property="og:image" content="{image}"><meta property="og:image:type" content="image/png">'
                '<meta property="og:image:width" content="1200"><meta property="og:image:height" content="630">'
                '<meta property="og:image:alt" content="Kinosail Player Docs sail mark">'
                f'<meta name="twitter:image" content="{image}"><meta name="twitter:card" content="summary_large_image">'
                f'<script type="application/ld+json">{json.dumps(data)}</script>')

    def test_canonical_and_schema_agree(self):
        self.assertEqual(validate(self.fixture(), 'https://example.org/kinosail/'), [])

    def test_missing_duplicate_malformed_and_wrong_metadata_fail(self):
        valid = self.fixture()
        for source in ['', valid + valid, valid.replace('Media library', ''), valid.replace('"@graph"', '"other"'), valid.replace('https://example.org', 'http://example.org'), valid.replace('{"@context"', '{broken"@context"')]:
            with self.subTest(source=source):
                self.assertTrue(validate(source, 'https://example.org/kinosail/'))

    def test_social_image_missing_malformed_or_conflicting_fails(self):
        valid = self.fixture()
        image = '<meta property="og:image" content="https://example.org/assets/share.png">'
        for source in (valid.replace(image, ''), valid.replace(image, image + image),
                       valid.replace('https://example.org/assets/share.png', 'https://other.example/share.png', 1),
                       valid.replace('content="1200"', 'content="0"'),
                       valid.replace('content="1200"', 'content="' + '9' * 10000 + '"'),
                       valid.replace('content="Kinosail Player Docs sail mark"', 'content=""'),
                       valid.replace('content="summary_large_image"', 'content="summary"'),
                       valid.replace('<meta name="twitter:image" content="https://example.org/assets/share.png">', '')):
            with self.subTest(source=source):
                self.assertTrue(validate(source, 'https://example.org/kinosail/'))
