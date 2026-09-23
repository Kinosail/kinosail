"""Validate crawlable pages' canonical URLs and structured data after rendering."""
from html.parser import HTMLParser
import json


class SearchMetadata(HTMLParser):
    def __init__(self, source):
        super().__init__()
        self.canonicals = []
        self.descriptions = []
        self.blocks = []
        self.current = None
        self.feed(source)

    def handle_starttag(self, tag, attributes):
        attrs = dict(attributes)
        if tag == 'link' and attrs.get('rel') == 'canonical':
            self.canonicals.append(attrs.get('href', ''))
        if tag == 'meta' and attrs.get('name') == 'description':
            self.descriptions.append(attrs.get('content', ''))
        if tag == 'script' and attrs.get('type') == 'application/ld+json':
            self.current = ''

    def handle_data(self, data):
        if self.current is not None:
            self.current += data

    def handle_endtag(self, tag):
        if tag == 'script' and self.current is not None:
            self.blocks.append(self.current)
            self.current = None


def validate(source, expected):
    page = SearchMetadata(source)
    errors = []
    if page.canonicals != [expected]:
        errors.append('canonical must match the public page URL exactly once')
    if len(page.descriptions) != 1 or not page.descriptions[0].strip():
        errors.append('one nonempty description required')
    if not page.blocks:
        errors.append('structured data missing')
    for block in page.blocks:
        try:
            data = json.loads(block)
            graph = data['@graph']
            if data['@context'] != 'https://schema.org' or not any(
                node.get('@type') == 'WebPage' and node.get('url') == expected
                for node in graph
            ):
                errors.append('structured data must identify this WebPage')
        except (ValueError, TypeError, KeyError, AttributeError):
            errors.append('invalid structured data')
    return errors
