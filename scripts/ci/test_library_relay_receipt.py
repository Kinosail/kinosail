"""Actual owned fixture caller retains sanitized relay facts before private cleanup."""
import json,unittest
import test_library_fixture_owner as peers
class RelayReceipt(unittest.TestCase):
    def setUp(self):
        self.peer=peers.LibraryFixtureOwnerTests();self.peer.setUp();self.addCleanup(self.peer.doCleanups)

    def test_owned_relay_close_retains_closed_transport_before_workspace_cleanup(self):
        codes=('ECONNRESET','ECONNREFUSED','ETIMEDOUT','EPIPE','ENETUNREACH','EHOSTUNREACH','unknown')
        transport=dict(schemaVersion=1,connections=1,connected=1,capacityRejected=0,connectDeadline=0,clientClosed=1,upstreamClosed=1,clientBytes=25,upstreamBytes=0,overflow=False,errors={side:{code:0 for code in codes} for side in ('client','upstream')})
        facts=dict(schemaVersion=1,containerRunning=True,networkInternal=True,targetAdmitted=True,transport=transport)
        result=self.peer.run_caller(CONTROL_RELAY_TRANSPORT=json.dumps(facts))
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(json.loads((self.peer.output/'relay-transport.json').read_text()),dict(schemaVersion=1,available=True,transport=transport))
        self.peer.assert_cleanup(self.peer.rows())

