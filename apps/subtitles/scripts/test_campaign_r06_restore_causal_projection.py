"""Test-first isolated gap: secret injection and inference rejection cannot be
covered by a populated Restore journey. Original acceptance remains independent."""
import copy
import unittest
import campaign_r06_restore_runtime_projection as projection

class CausalProjectionControls(unittest.TestCase):
    def value(self):
        native = dict(outcome='fulfilled',status=204,signalPresent=False,signalAborted=False,pagehide=False)
        network = dict(terminal='finished',failureCode='none',resourceType='Fetch',cancelled=False,navigation=False)
        server = dict(seen=True,held=False,delivered=True,cancelled=False,timedOut=False,settled=True)
        probes = [dict(mode=mode,native=copy.deepcopy(native),network=copy.deepcopy(network),server=copy.deepcopy(server)) for mode in ('direct','captured','held')]
        probes[2]['native'].update(outcome='rejected',status=0,signalPresent=True,signalAborted=True)
        probes[2]['network'].update(terminal='request-failed',failureCode='aborted',cancelled=True)
        probes[2]['server'].update(held=True,delivered=False,cancelled=True)
        return dict(schema='r06-causal-v1',valid=True,reason='none',headersEqual=True,framingEqual=True,stopped=True,
                    probes=probes,restoreNative=native,restoreNetwork=network)

    def test_exact_closed_projection_and_null_setup_admit(self):
        self.assertTrue(projection.valid_causal(None))
        self.assertTrue(projection.valid_causal(self.value()))

    def test_secret_shape_type_enum_and_duplicate_probe_rejected(self):
        for changes in ({'rawURL':'private'}, {'valid':1}, {'reason':'private error'}, {'probes':[]}):
            value=self.value();value.update(changes);self.assertFalse(projection.valid_causal(value))
        value=self.value();value['probes'][0]['network']['requestID']='private';self.assertFalse(projection.valid_causal(value))
        value=self.value();value['probes'][0]['mode']='held';self.assertFalse(projection.valid_causal(value))

    def test_invalid_control_never_qualifies_causal_inference(self):
        for part,key,value in ((None,'stopped',False),(None,'headersEqual',False),(None,'framingEqual',False),
                               ('held','cancelled',False),('held','timedOut',True),('direct','settled',False)):
            item=self.value()
            if part is None:item[key]=value
            else:next(p for p in item['probes'] if p['mode']==part)['server'][key]=value
            self.assertFalse(projection.causal_ready(item))

    def test_diagnostic_never_changes_strict_restore_acceptance(self):
        from test_campaign_r06_restore_runtime_projection import RestoreBrowserProjectionControls
        helper=RestoreBrowserProjectionControls();value=helper.value()
        for row in value['cases']:
            row['data']['causalDiagnostic']=self.value()
            row['data'].update(restoreTerminal='request-failed',restoreFailureCode='aborted',restoreClientCancelled=False)
        self.assertEqual(projection.classify(value,'a'*64),'prerequisite-blocked')

    def test_four_requests_require_complete_matched_native_and_cdp_outcomes(self):
        for field,key,changed in (('restoreNative','outcome','unreached'),('restoreNative','outcome','pending'),
            ('restoreNetwork','terminal','pending'),('restoreNetwork','resourceType','Other'),
            ('restoreNative','status',0),('restoreNative','signalAborted',True)):
            value=self.value();value[field][key]=changed
            self.assertFalse(projection.causal_ready(value))
        value=self.value();value['probes'][0]['native']['signalAborted']=True
        self.assertFalse(projection.causal_ready(value))

    def test_current_diagnostic_inputs_are_explicit_and_historical_binding_remains(self):
        from test_campaign_r06_restore_runtime_selection import RestoreSelectionControls
        import campaign_r06_restore_runtime_sources as sources
        helper=RestoreSelectionControls();value=helper.startup_package()
        paths=sorted(sources.CAUSAL_INPUTS)
        value['causalDiagnosticTransition']={'baseline':'96761b17b8ad495378b582e1d629e32090801062','addedInputs':paths}
        value['inputs'] += [dict(path=path,bytes=0,sha256='0'*64,blob='0'*40) for path in paths]
        self.assertEqual(helper.read_startup_package(value),value)
        value['inputs'].pop()
        with self.assertRaises(ValueError):helper.read_startup_package(value)
