"""Descriptor intake controls; synthetic bytes only, no application build."""
import hashlib
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

HELPER = Path(__file__).resolve().parents[1] / 'e2e/recovery-data-digest.py'

class DataDigest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        (self.root / 'data').mkdir()
        self.fd = os.open(self.root, os.O_RDONLY | os.O_DIRECTORY)
        self.addCleanup(self.tmp.cleanup)
        self.addCleanup(os.close, self.fd)

    def module(self):
        spec = importlib.util.spec_from_file_location('digest', HELPER)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def process(self, fd=None):
        owned = self.fd if fd is None else fd
        return subprocess.run([sys.executable, '-I', '-c',
            "import os,runpy,sys;os.dup2(int(sys.argv[1]),3);p=sys.argv[2];sys.argv=[p];runpy.run_path(p,run_name='__main__')",
            str(owned), str(HELPER)], pass_fds=(owned,), capture_output=True, timeout=3)

    def test_process_digest_and_utf8_name(self):
        (self.root / 'data/é.txt').write_bytes(b'owned')
        result = self.process()
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout, hashlib.sha256('é.txt\0'.encode()+b'owned\0').hexdigest().encode()+b'\n')
        self.assertEqual(result.stderr, b'')

    def test_bad_root_and_absent_descriptor_closed(self):
        file = self.root / 'file'; file.write_bytes(b'private')
        fd = os.open(file, os.O_RDONLY)
        try: result = self.process(fd)
        finally: os.close(fd)
        self.assertNotEqual(result.returncode, 0); self.assertEqual(result.stdout+result.stderr, b'')
        result = subprocess.run([sys.executable, '-I', str(HELPER)], capture_output=True, timeout=3)
        self.assertNotEqual(result.returncode, 0); self.assertEqual(result.stdout+result.stderr, b'')

    def test_nonprivate_root_rejected_without_reads(self):
        module=self.module()
        os.chmod(self.root,0o755)
        with patch.object(module.os,'read',side_effect=AssertionError('read rejected root')):
            with self.assertRaises(ValueError): module.data_digest(self.fd)

    def test_rejected_types_and_bounds_no_reads(self):
        module = self.module()
        for mode in ('directory-link', 'file-link', 'fifo', 'hardlink', 'oversize', 'count'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as folder:
                root = Path(folder); (root/'data').mkdir(); (root/'foreign').write_bytes(b'private')
                if mode == 'directory-link':
                    (root/'data').rmdir(); (root/'data').symlink_to(root, target_is_directory=True)
                elif mode == 'file-link': (root/'data/link').symlink_to(root/'foreign')
                elif mode == 'fifo': os.mkfifo(root/'data/fifo')
                elif mode == 'hardlink': os.link(root/'foreign', root/'data/link')
                elif mode == 'oversize':
                    with (root/'data/large').open('wb') as out: out.truncate(33554433)
                else:
                    for i in range(257): (root/'data'/str(i)).touch()
                fd = os.open(root, os.O_RDONLY|os.O_DIRECTORY)
                try:
                    with patch.object(module.os, 'read', side_effect=AssertionError('read rejected input')):
                        with self.assertRaises((ValueError, OSError)): module.data_digest(fd)
                finally: os.close(fd)

    def test_directory_and_file_replacement_never_read_foreign(self):
        module = self.module(); real_open = os.open; real_read = os.read
        for directory in (True, False):
            with self.subTest(directory=directory), tempfile.TemporaryDirectory() as folder:
                root=Path(folder); (root/'data/nested').mkdir(parents=True)
                (root/'data/nested/item').write_bytes(b'owned')
                (root/'foreign').mkdir(); (root/'foreign/item').write_bytes(b'private')
                target=root/'data/nested' if directory else root/'data/nested/item'
                swapped=False; reads=[]
                def opening(name, flags, *args, **kwargs):
                    nonlocal swapped
                    if name == ('nested' if directory else 'item') and not swapped:
                        swapped=True; target.rename(target.with_name(target.name+'-old'))
                        target.symlink_to(root/'foreign' if directory else root/'foreign/item', target_is_directory=directory)
                    return real_open(name, flags, *args, **kwargs)
                def reading(fd, count): reads.append(fd); return real_read(fd,count)
                fd=real_open(root,os.O_RDONLY|os.O_DIRECTORY)
                try:
                    with patch.object(module.os,'open',side_effect=opening), patch.object(module.os,'read',side_effect=reading):
                        with self.assertRaises((ValueError,OSError)): module.data_digest(fd)
                    self.assertTrue(swapped); self.assertEqual(reads, [])
                finally: os.close(fd)

    def test_same_type_inode_replacement_and_content_change_reject(self):
        module=self.module(); real_open=os.open; real_read=os.read
        item=self.root/'data/item'; item.write_bytes(b'owned')
        def opening(name,flags,*args,**kwargs):
            if name=='item': item.rename(self.root/'saved'); item.write_bytes(b'private')
            return real_open(name,flags,*args,**kwargs)
        with patch.object(module.os,'open',side_effect=opening), patch.object(module.os,'read',side_effect=AssertionError('foreign read')):
            with self.assertRaises(ValueError): module.data_digest(self.fd)
        item.write_bytes(b'owned')
        for growth in (False,True):
            item.write_bytes(b'owned'); changed=False
            def reading(fd,count):
                nonlocal changed
                body=real_read(fd,count)
                if not changed:
                    changed=True; item.write_bytes(b'changed' if growth else b'other')
                return body
            with patch.object(module.os,'read',side_effect=reading):
                with self.assertRaises(ValueError): module.data_digest(self.fd)

if __name__ == '__main__': unittest.main()
