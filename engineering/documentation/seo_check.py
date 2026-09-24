"""Validate crawlable pages' canonical URLs and structured data after rendering."""
from html.parser import HTMLParser
import json
from urllib.parse import urlsplit

SOCIAL_KEYS = ('og:image', 'og:image:type', 'og:image:width', 'og:image:height',
               'og:image:alt', 'twitter:image', 'twitter:card')


class SearchMetadata(HTMLParser):
    def __init__(self, source):
        super().__init__()
        self.canonicals = []
        self.descriptions = []
        self.social = {}
        self.blocks = []
        self.current = None
        self.breadcrumbs_visible = False
        self.feed(source)

    def handle_starttag(self, tag, attributes):
        attrs = dict(attributes)
        if tag == 'nav' and 'breadcrumbs' in attrs.get('class', '').split():
            self.breadcrumbs_visible = True
        if tag == 'link' and attrs.get('rel') == 'canonical':
            self.canonicals.append(attrs.get('href', ''))
        if tag == 'meta' and attrs.get('name') == 'description':
            self.descriptions.append(attrs.get('content', ''))
        if tag == 'meta':
            key = attrs.get('property') or attrs.get('name')
            if key in SOCIAL_KEYS:
                self.social.setdefault(key, []).append(attrs.get('content', ''))
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
    for key in SOCIAL_KEYS:
        if len(page.social.get(key, [])) != 1 or not page.social[key][0].strip():
            errors.append(f'one nonempty {key} required')
    image = page.social.get('og:image', [''])[0]
    try:
        parsed = urlsplit(image)
        image_valid = (parsed.scheme == 'https' and parsed.netloc == urlsplit(expected).netloc
                       and bool(parsed.path) and not parsed.query and not parsed.fragment)
    except ValueError:
        image_valid = False
    if image and not image_valid:
        errors.append('social image must use an absolute same-origin HTTPS URL')
    if page.social.get('twitter:image') and page.social['twitter:image'][0] != image:
        errors.append('Twitter image must match Open Graph image')
    if page.social.get('twitter:card') and page.social['twitter:card'][0] != 'summary_large_image':
        errors.append('Twitter card must use the large image')
    for key in ('og:image:width', 'og:image:height'):
        value = page.social.get(key, [''])[0]
        if value and (len(value) > 4 or not value.isascii() or not value.isdecimal() or not 0 < int(value) <= 8192):
            errors.append(f'{key} must be a positive image dimension')
    if not page.blocks:
        errors.append('structured data missing')
    breadcrumbs = []
    for block in page.blocks:
        try:
            data = json.loads(block)
            graph = data['@graph']
            if data['@context'] != 'https://schema.org' or not any(
                node.get('@type') == 'WebPage' and node.get('url') == expected
                for node in graph
            ):
                errors.append('structured data must identify this WebPage')
            breadcrumbs.extend(node for node in graph if node.get('@type') == 'BreadcrumbList')
        except (ValueError, TypeError, KeyError, AttributeError):
            errors.append('invalid structured data')
    if page.breadcrumbs_visible:
        if len(breadcrumbs) != 1:
            errors.append('visible breadcrumbs require one BreadcrumbList')
        else:
            items = breadcrumbs[0].get('itemListElement')
            if (not isinstance(items, list) or len(items) < 2 or
                any(not isinstance(item, dict) or item.get('@type') != 'ListItem' or
                    item.get('position') != position or not item.get('name') or
                    not isinstance(item.get('item'), str) or
                    not item['item'].startswith(urlsplit(expected).scheme + '://' + urlsplit(expected).netloc + '/')
                    for position, item in enumerate(items, 1)) or
                items[-1].get('item') != expected):
                errors.append('BreadcrumbList must follow the visible page hierarchy')
    return errors
