"""Exercise fixed Camera fixture admission with the existing synthetic command peers."""
import json
import unittest

import test_library_fixture_owner as fixture_peers
from test_library_profile_admission import load, report


class CameraFixtureOwnerTests(unittest.TestCase):
    def peer(self):
        peer = fixture_peers.LibraryFixtureOwnerTests()
        peer.setUp()
        self.addCleanup(peer.temporary.cleanup)
        peer.discovery.write_text(json.dumps(report(load().CAMERA_CASES, 'chromium', False)))
        return peer

    def test_fixed_camera_profile_owns_one_server_and_owner(self):
        peer = self.peer()
        result = peer.run_caller(['chromium', str(peer.discovery), str(peer.output), 'camera-fake'])
        self.assertEqual(result.returncode, 0, result.stderr)
        rows = peer.rows()
        self.assertEqual(rows[0][0], 'admit')
        owner = [args for kind, args in rows if kind == 'owner']
        self.assertEqual(len(owner), 1)
        self.assertEqual(owner[0][owner[0].index('--profile') + 1], 'camera-fake')
        peer.assert_cleanup(rows)

    def test_unknown_or_ambiguous_profile_rejects_before_all_fixture_effects(self):
        peer = self.peer()
        for extra in ([''], ['camera'], ['camera-fake', 'extra'], ['x' * 1025]):
            result = peer.run_caller(['chromium', str(peer.discovery), str(peer.output), *extra])
            self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(peer.rows(), [])
        self.assertFalse(peer.output.exists())


if __name__ == '__main__':
    unittest.main()
