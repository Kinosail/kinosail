"""Reject invalid deployment inputs before invoking tools or writing output."""
import contextlib
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from build import ROOT, build, settings
from check import Page, check


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

    def test_robots_names_the_generated_sitemap_for_each_origin(self):
        with tempfile.TemporaryDirectory() as directory:
            for name, origin, prefix in (('production', 'https://kinosail.com', ''),
                                         ('preview', 'https://example.org', '/preview')):
                with self.subTest(name=name):
                    args = settings(['--output', directory + '/' + name, '--url', origin, '--baseurl', prefix])
                    build(args)
                    self.assertIn(f'Sitemap: {origin}{prefix}/sitemap.xml',
                                  (args.output / 'robots.txt').read_text().splitlines())
                    self.assertEqual((args.output / 'index.html').read_text().count(
                        '<meta name="google-site-verification" content="CyK7nEwl7e61vmJVjO4nsO4JpjaeKGUMffYcSNcUhFg">'), 1)
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
