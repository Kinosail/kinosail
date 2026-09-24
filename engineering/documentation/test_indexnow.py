"""Exercise changed-page selection and the outbound IndexNow boundary."""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import MagicMock, patch
from urllib.error import HTTPError
from xml.etree import ElementTree

from build import build, settings
import indexnow


class IndexNowTest(unittest.TestCase):
    def test_source_paths_match_production_sitemap(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'site'
            build(settings(['--output', str(output)]))
            sitemap = ElementTree.parse(output / 'sitemap.xml')
            published = {item.text for item in sitemap.iter('{http://www.sitemaps.org/schemas/sitemap/0.9}loc')}
            source = {url for path in (indexnow.ROOT / indexnow.SOURCE).rglob('*') if path.is_file()
                      if (url := indexnow.page_url(str(path.relative_to(indexnow.ROOT))))}
            self.assertEqual(source, published)

    def test_diff_includes_changed_deleted_and_renamed_pages(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(indexnow, 'ROOT', Path(directory)):
            root = Path(directory)
            subprocess.run(['git', 'init', '-q'], cwd=root, check=True)
            docs = root / indexnow.SOURCE
            (docs / 'docs').mkdir(parents=True)
            for name in ('index.md', 'faq.md', 'docs/index.md'):
                (docs / name).write_text(name)
            before = self.commit(root)
            (docs / 'faq.md').write_text('updated answer')
            (docs / 'index.md').unlink()
            (docs / 'guide').mkdir()
            (docs / 'docs/index.md').rename(docs / 'guide/index.md')
            after = self.commit(root)
            self.assertEqual(indexnow.changed_urls(before, after),
                             ['https://kinosail.com/', 'https://kinosail.com/docs/',
                              'https://kinosail.com/faq/', 'https://kinosail.com/guide/'])
            (docs / '_includes').mkdir()
            (docs / '_includes/seo.html').write_text('new shared metadata')
            global_after = self.commit(root)
            self.assertEqual(indexnow.changed_urls(after, global_after),
                             ['https://kinosail.com/faq/', 'https://kinosail.com/guide/'])

    @staticmethod
    def commit(root):
        subprocess.run(['git', 'add', '-A'], cwd=root, check=True)
        subprocess.run(['git', '-c', 'user.name=Test', '-c', 'user.email=test@example.invalid',
                        'commit', '-qm', 'docs'], cwd=root, check=True)
        return subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()

    def test_invalid_revisions_and_paths_cannot_notify(self):
        with patch.object(indexnow, 'notify') as notify:
            for before, after in (('', 'a' * 40), ('x' * 40, 'a' * 40),
                                  ('a' * 40, 'b' * 41), ('a' * 39, 'b' * 40)):
                with self.subTest(before=before, after=after), self.assertRaises(ValueError):
                    indexnow.changed_urls(before, after)
            notify.assert_not_called()
        for path in ('apps/player/docs/../secret.md', 'apps/player/docs/Bad Page.md',
                     'apps/player/docs/' + 'a' * 513 + '.md'):
            with self.subTest(path=path), self.assertRaises(ValueError):
                indexnow.page_url(path)
        self.assertIsNone(indexnow.page_url('apps/player/docs/_includes/seo.html'))
        self.assertIsNone(indexnow.page_url('apps/player/docs/404.md'))

    def test_submission_contains_only_validated_public_urls(self):
        response = MagicMock()
        response.__enter__.return_value.status = 200
        with patch('indexnow.urlopen', return_value=response) as send:
            indexnow.notify(['https://kinosail.com/faq/'])
            request = send.call_args.args[0]
            self.assertEqual(send.call_args.kwargs['timeout'], 15)
            self.assertEqual(json.loads(request.data), {
                'host': 'kinosail.com', 'key': indexnow.public_key(),
                'keyLocation': f'https://kinosail.com/{indexnow.public_key()}.txt',
                'urlList': ['https://kinosail.com/faq/'],
            })
        with patch('indexnow.urlopen') as send:
            indexnow.notify([])
            send.assert_not_called()

    def test_rejected_payloads_make_no_request(self):
        good = 'https://kinosail.com/faq/'
        for urls in ([good, good], ['https://elsewhere.example/faq/'],
                     ['https://kinosail.com/faq/?x=1'], ['https://kinosail.com/../secret/'],
                     ['https://kinosail.com/' + 'a' * 513 + '/'], [good] * 10_001):
            with self.subTest(urls=urls[:2]), patch('indexnow.urlopen') as send:
                with self.assertRaises(ValueError):
                    indexnow.notify(urls)
                send.assert_not_called()
        with patch('indexnow.urlopen', side_effect=HTTPError(indexnow.ENDPOINT, 403,
                                                               'Forbidden', None, None)):
            with self.assertRaisesRegex(RuntimeError, 'HTTP 403'):
                indexnow.notify([good])

    def test_invalid_key_file_cannot_send(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(indexnow, 'ROOT', Path(directory)):
            source = Path(directory) / indexnow.SOURCE
            source.mkdir(parents=True)
            key_file = source / ('a' * 32 + '.txt')
            for content in (None, 'mismatch', 'a' * 130):
                if content is not None:
                    key_file.write_text(content)
                with self.subTest(content=content), patch('indexnow.urlopen') as send:
                    with self.assertRaises(ValueError):
                        indexnow.notify(['https://kinosail.com/faq/'])
                    send.assert_not_called()


if __name__ == '__main__':
    unittest.main()
