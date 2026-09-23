import json
import unittest
from seo_check import validate


class SearchMetadataTest(unittest.TestCase):
    def fixture(self, url='https://example.org/kinosail/'):
        data = {'@context': 'https://schema.org', '@graph': [{'@type': 'WebPage', 'url': url}]}
        return f'<link rel="canonical" href="{url}"><meta name="description" content="Media library"><script type="application/ld+json">{json.dumps(data)}</script>'

    def test_canonical_and_schema_agree(self):
        self.assertEqual(validate(self.fixture(), 'https://example.org/kinosail/'), [])

    def test_missing_duplicate_malformed_and_wrong_metadata_fail(self):
        valid = self.fixture()
        for source in ['', valid + valid, valid.replace('Media library', ''), valid.replace('"@graph"', '"other"'), valid.replace('https://example.org', 'http://example.org'), valid.replace('{"@context"', '{broken"@context"')]:
            with self.subTest(source=source):
                self.assertTrue(validate(source, 'https://example.org/kinosail/'))
