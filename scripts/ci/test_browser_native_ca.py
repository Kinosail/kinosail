"""Native trust peers exercise the real install/remove ownership boundary, not browsers."""
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / 'scripts/ci/browser-native-ca.py'

class NativeTrust(unittest.TestCase):
    def setUp(self):
        if not HELPER.exists(): self.skipTest('written-first helper not implemented')
        spec = importlib.util.spec_from_file_location('native_ca', HELPER)
        self.module = importlib.util.module_from_spec(spec); spec.loader.exec_module(self.module)
        self.temp = tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve(); self.home = self.root / 'home'; self.home.mkdir()
        self.workspace = self.root / 'fixture'; self.workspace.mkdir()
        self.cert = self.workspace / 'browser-fixture-ca.crt'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256', '-nodes', '-keyout', str(self.root/'key'), '-out', str(self.cert), '-days', '1', '-subj', '/CN=Test CA', '-addext', 'basicConstraints=critical,CA:TRUE'], check=True, capture_output=True)
        self.firefox = self.home / '.cache/ms-playwright/firefox-1543/firefox/firefox'
        self.firefox.parent.mkdir(parents=True); self.firefox.write_text('owned fixture executable'); self.firefox.chmod(0o700)
        self.effects = []; self.certs = {}; self.fail_import = False
        self.real_run = self.module.run
        def run(argv, data=None, **kwargs):
            if argv[0] != 'certutil': return self.real_run(argv, data, **kwargs)
            self.effects.append(argv)
            directory = Path(argv[argv.index('-d') + 1][4:])
            if '-N' in argv:
                for name in ('cert9.db', 'key4.db', 'pkcs11.txt'): (directory/name).write_text('owned database')
                return b''
            if '-A' in argv:
                self.certs[argv[argv.index('-n')+1]] = Path(argv[argv.index('-i')+1]).read_bytes()
                if self.fail_import: raise ValueError('tool failure after import')
                return b''
            if '-D' in argv: del self.certs[argv[argv.index('-n')+1]]; return b''
            if '-n' in argv: return self.certs[argv[argv.index('-n')+1]]
            return ('Certificate Nickname        Trust Attributes\n' + ''.join(name+'        C,,\n' for name in self.certs)).encode()
        for mock in (patch.object(Path, 'home', return_value=self.home), patch.object(self.module, 'firefox_executable', return_value=self.firefox), patch.object(self.module, 'run', side_effect=run), patch.object(self.module.sys, 'platform', 'linux'), patch.dict(os.environ, {'CI':'true','GITHUB_ACTIONS':'true','RUNNER_OS':'Linux'}), patch.object(self.module.os, 'urandom', return_value=bytes.fromhex('a'*32))):
            mock.start(); self.addCleanup(mock.stop)

    def install(self, project='chromium', **kwargs):
        return self.module.install(project, kwargs.get('certificate',self.cert), self.workspace, kwargs.get('nonce','123-456'))

    def remove(self, project='chromium'):
        return self.module.remove(project, self.cert, self.workspace, '123-456')

    def test_unknown_missing_and_malformed_inputs_have_no_native_effect(self):
        for project, nonce in [('unknown','123-456'), ('','123-456'), ('chromium','../x'), ('chromium','x'*33)]:
            with self.subTest(project=project,nonce=nonce):
                with self.assertRaises(ValueError): self.install(project,nonce=nonce)
                self.assertEqual(self.effects, [])
        foreign = self.root / 'foreign.crt'; foreign.write_bytes(self.cert.read_bytes())
        with self.assertRaises(ValueError): self.install(certificate=foreign)
        self.assertEqual(self.effects, [])

    def test_bad_certificate_and_symlink_reject_before_trust_mutation(self):
        original=self.cert.read_bytes()
        for value in (b'', b'PRIVATE KEY', b'x'*262145, original+b'unknown\n'):
            self.cert.write_bytes(value)
            with self.assertRaises(ValueError): self.install()
            self.assertEqual(self.effects, [])
        self.cert.unlink(); self.cert.symlink_to(self.root/'missing')
        with self.assertRaises(ValueError): self.install()
        self.assertEqual(self.effects, [])

    def test_chromium_new_database_and_certificate_are_owned_and_joined(self):
        self.install(); self.assertEqual(len(self.certs),1)
        directory=self.home/'.local/share/pki/nssdb'; self.assertTrue(directory.exists())
        self.remove(); self.remove(); self.assertEqual(self.certs,{})
        self.assertFalse(directory.exists())

    def test_chromium_legacy_database_preserves_unrelated_certificates(self):
        directory=self.home/'.pki/nssdb'; directory.mkdir(parents=True)
        for name in ('cert9.db','key4.db','pkcs11.txt'): (directory/name).write_text('foreign database')
        self.certs['foreign CA']=b'foreign certificate'
        self.install(); self.remove()
        self.assertEqual(self.certs,{'foreign CA':b'foreign certificate'})
        self.assertTrue(directory.exists())
        self.assertTrue(all('sql:'+str(directory) in argv for argv in self.effects))

    def test_symlinked_native_database_rejects_without_effect(self):
        parent=self.home/'.pki'; parent.mkdir(); (parent/'nssdb').symlink_to(self.workspace,target_is_directory=True)
        with self.assertRaises(ValueError): self.install()
        self.assertEqual(self.effects,[])

    def test_foreign_nickname_is_not_adopted_or_removed(self):
        directory=self.home/'.pki/nssdb'; directory.mkdir(parents=True)
        for name in ('cert9.db','key4.db','pkcs11.txt'): (directory/name).write_text('foreign')
        self.certs['kinosail-fixture-'+'a'*32]=b'foreign'
        with self.assertRaises(ValueError): self.install()
        self.remove(); self.assertEqual(list(self.certs.values()),[b'foreign'])

    def test_partial_success_import_failure_still_has_owned_cleanup(self):
        self.fail_import=True
        with self.assertRaises(ValueError): self.install()
        self.assertEqual(len(self.certs),1)
        self.remove(); self.assertEqual(self.certs,{})

    def test_cleanup_rejects_changed_certificate_without_removing_it(self):
        self.install(); name=next(iter(self.certs)); self.certs[name]=b'foreign'
        with self.assertRaises(ValueError): self.remove()
        self.assertEqual(self.certs[name],b'foreign')

    def test_firefox_policy_owned_install_cleanup_and_existing_file_preservation(self):
        self.install('firefox'); policy=self.firefox.parent/'distribution/policies.json'
        self.assertIn(str(self.cert),policy.read_text()); self.remove('firefox'); self.assertFalse(policy.exists())
        policy.parent.mkdir(exist_ok=True); policy.write_text('{"foreign":true}')
        with self.assertRaises(ValueError): self.install('firefox')
        self.remove('firefox'); self.assertEqual(policy.read_text(),'{"foreign":true}')

    def test_firefox_cleanup_rejects_replaced_policy_and_directory(self):
        for swap in ('file','directory'):
            with self.subTest(swap=swap):
                self.install('firefox'); policy=self.firefox.parent/'distribution/policies.json'
                if swap=='file': policy.unlink(); policy.write_text('foreign')
                else:
                    moved=policy.parent.with_name('retained-policy'); policy.parent.rename(moved); policy.parent.mkdir(); policy.write_text('foreign')
                with self.assertRaises(ValueError): self.remove('firefox')
                self.assertEqual(policy.read_text(),'foreign')
                break  # separate directory negative below avoids overwriting a failed owned receipt

    def test_firefox_cleanup_rejects_replaced_parent(self):
        self.install('firefox'); directory=self.firefox.parent/'distribution'
        directory.rename(directory.with_name('retained')); directory.mkdir(); (directory/'policies.json').write_text('foreign')
        with self.assertRaises(ValueError): self.remove('firefox')
        self.assertEqual((directory/'policies.json').read_text(),'foreign')

    def test_actual_tool_boundary_caps_stdout_stderr_and_deadline(self):
        import sys
        for code in ("import os;os.write(1,b'x'*65537)", "import os;os.write(2,b'x'*8193)", "import time;time.sleep(20)"):
            with self.subTest(code=code):
                with self.assertRaises(ValueError): self.real_run([sys.executable, '-c', code])

    def test_partial_state_and_unknown_fields_cannot_delete_foreign_resources(self):
        import json
        self.install('firefox'); receipt=self.workspace/'browser-native-trust-123-456.json'
        data=json.loads(receipt.read_text()); data['unknown']='foreign'; receipt.write_text(json.dumps(data))
        with self.assertRaises(ValueError): self.remove('firefox')
        self.assertTrue((self.firefox.parent/'distribution/policies.json').exists())

    def test_foreign_certificate_added_to_new_database_retains_its_files(self):
        self.install(); self.certs['foreign CA']=b'foreign'
        with self.assertRaises(ValueError): self.remove()
        self.assertEqual(self.certs,{'foreign CA':b'foreign'})
        self.assertTrue((self.home/'.local/share/pki/nssdb/cert9.db').exists())

    def test_firefox_resolver_uses_actual_pinned_dependency_directory(self):
        original=__import__('importlib.util',fromlist=['spec_from_file_location'])
        spec=original.spec_from_file_location('resolver_copy',HELPER); module=original.module_from_spec(spec); spec.loader.exec_module(module)
        observed=[]
        def command(argv, **kwargs): observed.append(kwargs.get('cwd')); return str(self.firefox).encode()
        with patch.object(module,'run',side_effect=command):
            self.assertEqual(module.firefox_executable(),self.firefox)
        self.assertEqual(observed,[ROOT/'apps/player/e2e'])

    def test_replaced_install_receipt_is_never_truncated(self):
        original=self.module.run
        receipt=self.workspace/'browser-native-trust-123-456.json'
        def command(argv, *args, **kwargs):
            result=original(argv,*args,**kwargs)
            if argv[0]=='certutil' and '-N' in argv:
                receipt.rename(receipt.with_suffix('.retained')); receipt.write_text('foreign receipt')
            return result
        with patch.object(self.module,'run',side_effect=command):
            with self.assertRaises(ValueError): self.install()
        self.assertEqual(receipt.read_text(),'foreign receipt')

    def test_replaced_install_policy_directory_has_no_foreign_write(self):
        original=self.module.write_state
        calls=0
        def write(*args,**kwargs):
            nonlocal calls
            result=original(*args,**kwargs); calls+=1
            if calls==2:
                directory=self.firefox.parent/'distribution'; directory.rename(directory.with_name('retained'))
                directory.mkdir()
            return result
        with patch.object(self.module,'write_state',side_effect=write):
            with self.assertRaises(ValueError): self.install('firefox')
        self.assertFalse((self.firefox.parent/'distribution/policies.json').exists())

    def test_cleanup_policy_parent_swap_cannot_unlink_foreign_policy(self):
        self.install('firefox'); directory=self.firefox.parent/'distribution'
        original=self.module.read_regular
        def read(path,*args,**kwargs):
            result=original(path,*args,**kwargs)
            if Path(path).name=='policies.json':
                directory.rename(directory.with_name('retained')); directory.mkdir()
                (directory/'policies.json').write_text('foreign policy')
            return result
        with patch.object(self.module,'read_regular',side_effect=read):
            with self.assertRaises(ValueError): self.remove('firefox')
        self.assertEqual((directory/'policies.json').read_text(),'foreign policy')

    def test_malformed_nss_receipt_rejects_before_certificate_removal(self):
        import json
        self.install(); receipt=self.workspace/'browser-native-trust-123-456.json'
        state=json.loads(receipt.read_text()); state['files']={'unknown':[1,2]}
        receipt.write_text(json.dumps(state)); before=self.certs.copy(); self.effects.clear()
        with self.assertRaises(ValueError): self.remove()
        self.assertEqual(self.certs,before);self.assertEqual(self.effects,[])
