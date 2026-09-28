"""Reject invalid deployment inputs before invoking tools or writing output."""
import contextlib
from html.parser import HTMLParser
import io
import json
from pathlib import Path
import struct
import tempfile
import unittest
from unittest.mock import patch

from build import ROOT, build, settings
from check import Page, check
from seo_check import SearchMetadata


class LinkText(HTMLParser):
    def __init__(self, source):
        super().__init__()
        self.links = []
        self.href = None
        self.feed(source)

    def handle_starttag(self, tag, attributes):
        if tag == 'a':
            self.href = dict(attributes).get('href')
            self.links.append([self.href, ''])

    def handle_data(self, data):
        if self.href is not None:
            self.links[-1][1] += data

    def handle_endtag(self, tag):
        if tag == 'a':
            self.href = None


class BuildInputsTest(unittest.TestCase):
    def test_defaults_target_the_root_custom_domain(self):
        with tempfile.TemporaryDirectory() as directory:
            args = settings(['--output', directory + '/new'])
        self.assertEqual(args.baseurl, '')
        self.assertEqual(args.url, 'https://kinosail.com')

    def test_valid_origins_and_prefixes(self):
        with tempfile.TemporaryDirectory() as directory:
            for prefix in ('', '/kinosail', '/preview/docs-v2'):
                args = settings(['--output', directory + '/new', '--baseurl', prefix])
                self.assertEqual(args.baseurl, prefix)
                self.assertFalse(args.output.exists())

    def test_public_discovery_files_for_each_origin(self):
        with tempfile.TemporaryDirectory() as directory:
            for name, origin, prefix in (('production', 'https://kinosail.com', ''),
                                         ('preview', 'https://example.org', '/preview')):
                with self.subTest(name=name):
                    args = settings(['--output', directory + '/' + name, '--url', origin, '--baseurl', prefix])
                    build(args)
                    self.assertIn(f'Sitemap: {origin}{prefix}/sitemap.xml',
                                  (args.output / 'robots.txt').read_text().splitlines())
                    self.assertEqual('<loc>https://kinosail.com/architecture-explorer/</loc>'
                                     in (args.output / 'sitemap.xml').read_text(), name == 'production')
                    homepage = (args.output / 'index.html').read_text()
                    for app_name in ('player', 'subtitles'):
                        self.assertEqual(
                            (args.output / f'assets/install/{app_name}.yaml').read_bytes(),
                            (ROOT / f'apps/{app_name}/packaging/platform-compose.yaml').read_bytes(),
                        )
                    self.assertEqual(
                        (args.output / 'assets/install/both.yaml').read_bytes(),
                        (ROOT / 'apps/player/packaging/platform-compose-both.yaml').read_bytes(),
                    )
                    install_helper = (args.output / 'getting-started/platforms/index.html').read_text()
                    self.assertIn('data-install-builder', install_helper)
                    self.assertIn(f'{prefix}/assets/js/platform-install.js', install_helper)
                    favicon = (args.output / 'assets/images/kinosail-mark.png').read_bytes()
                    self.assertEqual(favicon[:8], b'\x89PNG\r\n\x1a\n')
                    self.assertEqual(struct.unpack('>II', favicon[16:24]), (96, 96))
                    icon_link = (f'<link rel="icon" type="image/png" sizes="96x96" '
                                 f'href="{prefix}/assets/images/kinosail-mark.png">')
                    self.assertIn(icon_link, homepage)
                    self.assertIn(icon_link, (args.output / 'docs/index.html').read_text())
                    self.assertEqual(homepage.count(
                        '<meta name="google-site-verification" content="CyK7nEwl7e61vmJVjO4nsO4JpjaeKGUMffYcSNcUhFg">'), 1)
                    graph = json.loads(SearchMetadata(homepage).blocks[0])['@graph']
                    organization = next(item for item in graph if item['@type'] == 'Organization')
                    self.assertIn('Kinosail Player, a free, source-available media server', organization['description'])
                    self.assertEqual(organization['logo'], f'{origin}{prefix}/assets/images/kinosail-mark.svg')
                    self.assertIn('viewBox="0 0 512 512"', (args.output / 'assets/images/kinosail-mark.svg').read_text())
                    app = next(item for item in graph if item['@type'] == 'SoftwareApplication')
                    self.assertEqual(app['offers'], {'@type': 'Offer', 'price': 0})
                    subtitles = (args.output / 'subtitles/index.html').read_text()
                    self.assertIn(f'<link rel="canonical" href="{origin}{prefix}/subtitles/">', subtitles)
                    self.assertIn(f'<meta property="og:url" content="{origin}{prefix}/subtitles/">', subtitles)
                    self.assertIn('Kinosail Subtitles', subtitles)
                    self.assertIn('<meta property="og:title" content="Kinosail Subtitles · Self-hosted subtitle automation">', subtitles)
                    share_url = f'{origin}{prefix}/subtitles/assets/images/kinosail-subtitles-docs-share.png'
                    subtitles_install = (args.output / 'subtitles/getting-started/install/index.html').read_text()
                    self.assertIn(f'<meta property="og:url" content="{origin}{prefix}/subtitles/getting-started/install/">', subtitles_install)
                    for page in (subtitles, subtitles_install):
                        social = SearchMetadata(page).social
                        self.assertEqual(social['og:image'], [share_url])
                        self.assertEqual(social['twitter:image'], [share_url])
                        self.assertEqual(social['og:image:alt'], ['Kinosail Subtitles Docs mark and the words Find. Validate. Save.'])
                    self.assertEqual(SearchMetadata(subtitles).social['og:image:width'], ['1200'])
                    self.assertEqual(SearchMetadata(subtitles).social['og:image:height'], ['630'])
                    share_png = (args.output / 'subtitles/assets/images/kinosail-subtitles-docs-share.png').read_bytes()
                    self.assertEqual(share_png[:8], b'\x89PNG\r\n\x1a\n')
                    self.assertEqual(struct.unpack('>II', share_png[16:24]), (1200, 630))
                    self.assertIn(f'<loc>{origin}{prefix}/subtitles/</loc>',
                                  (args.output / 'sitemap.xml').read_text())
                    subtitles_index = json.loads((args.output / 'subtitles/search.json').read_text())
                    self.assertGreater(len(subtitles_index), 10)
                    self.assertEqual({item['product'] for item in subtitles_index}, {'Subtitles'})
                    self.assertTrue(all(item['url'].startswith(prefix + '/subtitles/') for item in subtitles_index))
                    subtitles_graph = json.loads(SearchMetadata(subtitles).blocks[0])['@graph']
                    subtitles_app = next(item for item in subtitles_graph if item['@type'] == 'SoftwareApplication')
                    self.assertEqual(subtitles_app['name'], 'Kinosail Subtitles')
                    self.assertEqual(subtitles_app['offers'], {'@type': 'Offer', 'price': 0})
                    self.assertIn('ghcr.io/kinosail/kinosail-subtitles:latest', subtitles_install)
                    self.assertIn('Kinosail Subtitles dashboard', subtitles)
                    why_links = LinkText((args.output / 'why/index.html').read_text()).links
                    self.assertIn([f'{prefix}/quickstart/', 'Try Kinosail Player'], why_links)
                    self.assertIn([f'{prefix}/subtitles/getting-started/install/', 'Try Kinosail Subtitles'], why_links)
                    self.assertIn(['https://github.com/Kinosail/kinosail/issues/new/choose', 'Share feedback ↗'], why_links)
                    self.assertIn(['https://github.com/Kinosail/kinosail/issues/new/choose', 'Report a bug'], why_links)
                    for guide in ('getting-started/first-setup/index.html', 'subtitles/getting-started/install/index.html'):
                        guide_links = LinkText((args.output / guide).read_text()).links
                        self.assertIn(['https://github.com/Kinosail/kinosail/issues/new/choose', 'Open an issue'], guide_links)
                    check(args.output, prefix)

    def test_invalid_inputs_have_no_side_effects(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'new'
            cases = [
                ['--baseurl', value] for value in ('/', '//host', '/a/', '/a/../b', '/a?b', '/a#b', '/a b', '/' + 'a' * 201)
            ] + [
                ['--url', value] for value in ('', 'http://example.org', 'https://x@y.org', 'https://example.org/', 'https://example.org:443', 'https://example.org?a', 'https://example.org#x', 'https://[', 'https://EXAMPLE.org', ' https://example.org', 'https://example.org\n', 'https://' + 'a' * 254)
            ] + [
                ['--output', value] for value in ('', ' ', directory, str(ROOT / 'new-output'), str(ROOT / '..' / ROOT.name / 'new-output'), 'a' * 4097)
            ] + [['--unknown', 'value'], ['--out', 'value']]
            for case in cases:
                with self.subTest(case=case), patch('build.subprocess.run') as run, contextlib.redirect_stderr(io.StringIO()):
                    with self.assertRaises(SystemExit):
                        settings(['--output', str(output), *case])
                    run.assert_not_called()
                    self.assertFalse(output.exists())
                    self.assertEqual(list(Path(directory).iterdir()), [])

    def test_subtitles_metadata_is_checked(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'site'
            build(settings(['--output', str(output)]))
            page = output / 'subtitles/index.html'
            page.write_text(page.read_text().replace('rel="canonical"', 'rel="alternate"'))
            with self.assertRaisesRegex(SystemExit, 'subtitles/: canonical'):
                check(output, '')

    def test_missing_output_is_rejected(self):
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            settings([])

    def test_link_parser_covers_assets_and_fragments(self):
        page = Page('<h1>Guide</h1><h2 id="setup">Setup</h2><a href="#setup">Go</a><img src="/icon.svg">')
        self.assertEqual(page.h1, 1)
        self.assertEqual(page.ids, {'setup'})
        self.assertEqual(page.links, ['#setup', '/icon.svg'])


if __name__ == '__main__':
    unittest.main()
