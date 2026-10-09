"""Source gates and owned POSIX mode controls; Go execution belongs to hosted CI."""
import os
from pathlib import Path
import tempfile
import unittest
ROOT=Path(__file__).resolve().parents[2]
class SharedSecurityFindings(unittest.TestCase):
    def test_filter_bounds_precede_conversion_pointer_and_privilege_effects(self):
        source=(ROOT/'packages/publicgateway/restrict_linux.go').read_text()
        guard='if len(filter) == 0 || len(filter) > 65535 {'
        self.assertIn(guard,source)
        for effect in ('uint16(len(filter))','&filter[0]','runtime.LockOSThread()','unix.Prctl('):
            self.assertLess(source.index(guard),source.index(effect))
        self.assertIn('return errors.New("invalid public gateway network filter")',source)
    def test_reexec_is_current_binary_with_fixed_selector(self):
        source=(ROOT/'packages/publicgateway/restrict_linux_test.go').read_text()
        self.assertIn('executable, err := os.Executable()',source)
        self.assertNotIn('exec.CommandContext(ctx, os.Args[0]',source)
        self.assertIn('exec.CommandContext(ctx, executable, "-test.run=^TestGatewayNetworkRestrictionAcrossAllThreads$")',source)
    def test_duplicate_cookie_still_rejects_and_uses_security_flags(self):
        source=(ROOT/'packages/quickconnect/browser_test.go').read_text()
        line=source.split('"duplicate cookie":',1)[1].split('"oversized cookie":',1)[0]
        for flag in ('Secure: true','HttpOnly: true','SameSite: http.SameSiteStrictMode'):
            self.assertIn(flag,line)
        self.assertIn('f.writes != 0 || len(f.broker.pending) != 1',source)
    def test_directory_mode_is_owner_only_and_traversable(self):
        source=(ROOT/'packages/remoteaccess/gateway.go').read_text()
        self.assertIn('os.Chmod(publicgateway.Directory, 0o700)',source)
        self.assertIn('!directory.IsDir() || directory.Mode()&os.ModeSymlink != 0',source)
        self.assertIn('#nosec G302',source)
        self.assertIn('filepath.Dir(path) != publicgateway.Directory',source)
        with tempfile.TemporaryDirectory() as folder:
            directory=Path(folder)/'private';directory.mkdir(mode=0o700)
            child=directory/'owned';child.write_bytes(b'owned')
            self.assertEqual(directory.stat().st_mode&0o777,0o700)
            self.assertTrue(os.access(directory,os.X_OK));self.assertEqual(child.read_bytes(),b'owned')
            directory.chmod(0o600)
            try:
                if os.getuid()!=0:self.assertFalse(os.access(directory,os.X_OK))
            finally:directory.chmod(0o700)
    def test_annotations_are_call_specific_and_global_rules_remain(self):
        source=(ROOT/'.golangci.yml').read_text()
        self.assertIn('- gosec',source)
        self.assertNotIn('G302',source);self.assertNotIn('G204',source)
        for path,rule in [('packages/remoteaccess/gateway.go','G302'),('packages/publicgateway/restrict_linux_test.go','G204')]:
            lines=[line for line in (ROOT/path).read_text().splitlines() if '#nosec' in line]
            self.assertEqual(len(lines),1);self.assertIn('#nosec '+rule,lines[0])
if __name__=='__main__':unittest.main()
