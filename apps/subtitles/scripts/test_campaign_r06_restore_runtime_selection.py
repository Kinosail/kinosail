"""Failure-first fixed Restore selection; no compiler/browser runs."""
import unittest
from campaign_r06_restore_runtime_sources import selection, commands, SUITES

class RestoreSelectionControls(unittest.TestCase):
    def env(self, **changes):
        value = {"GITHUB_ACTIONS": "true", "RUNNER_OS": "Linux", "GITHUB_SHA": "a"*40,
                 "CAMPAIGN_PROOF": "R06", "CAMPAIGN_R06_SUITE": "restore-controls"}
        value.update(changes)
        return value

    def test_only_three_frozen_restore_suites(self):
        self.assertEqual(tuple(SUITES), ("restore-controls", "restore-headers", "restore-inspect-body"))
        for suite in SUITES:
            self.assertEqual(selection(self.env(CAMPAIGN_R06_SUITE=suite), []), suite)

    def test_cli_and_foreign_proofs_fail_before_work(self):
        for argv in (["--static-check"], ["--output", "/tmp/foreign"], ["restore-headers"]):
            with self.assertRaises(ValueError):
                selection(self.env(), argv)
        for proof in ("Q47", "R18", "", None, True):
            with self.assertRaises(ValueError):
                selection(self.env(CAMPAIGN_PROOF=proof), [])

    def test_hosted_platform_revision_and_suite_are_closed(self):
        for changes in ({"GITHUB_ACTIONS": "false"}, {"RUNNER_OS": "Darwin"},
                        {"GITHUB_SHA": "A"*40}, {"GITHUB_SHA": "a"*39},
                        {"CAMPAIGN_R06_SUITE": "save-headers"}, {"CAMPAIGN_R06_SUITE": None}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                selection(self.env(**changes), [])

    def test_each_exact_pair_and_all_case_collection(self):
        self.assertEqual(SUITES["restore-controls"][0], ())
        self.assertEqual(SUITES["restore-headers"][0],
                         ("r06-restore-headers-desktop", "r06-restore-headers-phone"))
        self.assertEqual(SUITES["restore-inspect-body"][0],
                         ("r06-restore-inspect-body-desktop", "r06-restore-inspect-body-phone"))
        command = commands("restore-headers", "/owned/restore-fixture.test", "/owned")
        self.assertIn("subtitle-restore-recovery.config.ts", " ".join(command["collection"]))
        self.assertIn("--list", command["collection"])
        self.assertIn("--global-timeout=15000", command["collection"])
        self.assertIn("--global-timeout=230000", command["browser"])
        self.assertEqual(command["browser"][-2:], ["--grep", SUITES["restore-headers"][1]])

    def test_public_names_and_compiled_direct_execution_are_exact(self):
        command = commands("restore-controls", "/owned/restore-fixture.test", "/owned")
        expected = ("TestRestoreLegacySwapPublicControl|TestRestorePreparedReceiptPublicControl|"
                    "TestRestoreHeldHeadersPublicControl|TestRestoreHeldInspectionBodyPublicControl")
        self.assertIn("-test.run=^(" + expected + ")$", command["controls"])
        self.assertIn("-test.parallel=1", command["controls"])
        self.assertIn("-test.count=1", command["controls"])
        self.assertIn("-test.timeout=60s", command["controls"])
        self.assertEqual(command["compile"][:5], ["go", "test", "-c", "-p", "1"])

    def test_foreign_suite_and_binary_boundaries_reject(self):
        for suite, binary, root in (("save-body", "/owned/restore-fixture.test", "/owned"),
                                    ("restore-controls", "relative", "/owned"),
                                    ("restore-controls", "/foreign/restore-fixture.test", "/owned"),
                                    ("restore-controls", "/owned/other.test", "/owned")):
            with self.assertRaises(ValueError):
                commands(suite, binary, root)

    def test_official_full_playwright_titles_select_exact_pair(self):
        # Playwright grep uses project/file/describe/test title separated by spaces.
        import re
        titles = {
            "restore-headers": ("R06 Restore held headers releases desktop editor", "R06 Restore held headers releases phone editor"),
            "restore-inspect-body": ("R06 Restore held inspection body releases desktop editor", "R06 Restore held inspection body releases phone editor")}
        for suite, pair in titles.items():
            pattern = commands(suite, "/owned/restore-fixture.test", "/owned")["browser"][-1]
            for prefix in ("", "chromium subtitle-restore-recovery.spec.ts ", "chromium subtitle-restore-recovery.spec.ts R06 Restore "):
                for title in pair:
                    self.assertIsNotNone(re.search(pattern, prefix + title), (suite, prefix, title))
                for other_suite, other_pair in titles.items():
                    if other_suite != suite:
                        for title in other_pair: self.assertIsNone(re.search(pattern, prefix + title))
                for title in (pair[0]+" foreign", "prefix"+pair[0], pair[0].replace("desktop", "tablet")):
                    self.assertIsNone(re.search(pattern, prefix+title))

    def test_selected_direct_cli_uses_absolute_owned_config_from_repository_cwd(self):
        from pathlib import Path
        from campaign_r06_restore_runtime_sources import APP
        for suite in ("restore-headers", "restore-inspect-body"):
            value = commands(suite, "/owned/restore-fixture.test", "/owned")
            for phase in ("collection", "browser"):
                selected = value[phase][value[phase].index("--config") + 1]
                self.assertTrue(Path(selected).is_absolute())
                self.assertEqual(Path(selected), APP / "e2e/subtitle-restore-recovery.config.ts")

    def test_entry_reader_opens_nonblocking_before_special_file_fstat(self):
        import stat, sys, types
        from unittest.mock import patch
        if "campaign_r06_restore_runtime_entry" not in sys.modules:
            import runpy
            from pathlib import Path
            entry = types.SimpleNamespace(**runpy.run_path(str(Path(__file__).with_name("campaign-r06-restore-browser.py")), run_name="restore-entry-tests"))
        else: entry = sys.modules["campaign_r06_restore_runtime_entry"]
        def opened(path,flags):
            self.assertTrue(flags & entry.os.O_NONBLOCK)
            self.assertTrue(flags & entry.os.O_NOFOLLOW)
            return 17
        with patch.object(entry.os,"lstat",return_value=types.SimpleNamespace(st_mode=stat.S_IFDIR)), \
             patch.object(entry.os,"open",side_effect=opened), \
             patch.object(entry.os,"fstat",return_value=types.SimpleNamespace(st_mode=stat.S_IFIFO)), \
             patch.object(entry.os,"close") as closed,self.assertRaises(ValueError):
            entry.load_modules()
        closed.assert_called_once_with(17)

    def startup_package(self):
        import json
        return json.loads("{\"schema\":\"r06-restore-runtime-source-v1\",\"currentInputBaseline\":{\"commit\":\"2b816fa8484279aa906116446f305d8f49ceb66d\",\"tree\":\"d511ed8f7634e6c82c8ec847eadcb9a8ab39bf65\",\"kind\":\"published-startup-corrected-raw-inputs\",\"formatterQualification\":\"pending\",\"canonicalOutputs\":null},\"inputs\":[{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_assertions_test.go\",\"bytes\":6490,\"lines\":183,\"blob\":\"0c72a93cf599390ecfa42a1b08ebbbe7e0c8bd6b\",\"sha256\":\"82261355e73118d9cad07c3f6fa289ac29a0d6343efb8db669926b96087191c0\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_controls_test.go\",\"bytes\":4697,\"lines\":112,\"blob\":\"ef0599a3a672d09603f5619b69cb1eee3fd2a770\",\"sha256\":\"d7be68372506e6813b40b7a5b2c9080a53af634a8a0ab95e8369c1b18bddacdb\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_exchange_test.go\",\"bytes\":4926,\"lines\":157,\"blob\":\"96dceb7318df112838f2d409d8647e7a5bcfbc18\",\"sha256\":\"b694f5a9f2060daac8fcc08e77076030b6d3a34733b9eb2f38da557763a0bfa1\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_filesystem_test.go\",\"blob\":\"e330d8bb78d411b32565ccb6c98d626eba9c6230\",\"bytes\":7649,\"lines\":212,\"sha256\":\"37b2c37f8534cc220871f6892c2b0e76be9e77ad7e7683be561d24b322000685\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_http_test.go\",\"bytes\":6127,\"lines\":207,\"blob\":\"96e2101833c66403eeed164552e696f7d0c52cba\",\"sha256\":\"c1b6b0e0c6fecd90220d03fb14ecf9c5ed83fc656b680a0c194a3faa94462a73\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_main_test.go\",\"blob\":\"8b53d7a674a4099d5b318ae35d4fd0bff41d549e\",\"bytes\":9238,\"lines\":251,\"sha256\":\"09e1ca5c0277ba72237cfdbb9ad880cca048d13d01ec31ea55ae1b77e946a9d4\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_owner_enrollment_test.go\",\"bytes\":5019,\"lines\":132,\"blob\":\"8c570ea7cf23f1c60b01950fee8ba5122b64ccf0\",\"sha256\":\"d856b68f17840df1c7b8ccd857dbb4cd2e8397ae89307ef73401ba3fefb4913b\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_owner_test.go\",\"bytes\":1373,\"lines\":59,\"blob\":\"687b5b542433ad9f91faf2475039dbb1d9926f60\",\"sha256\":\"1fd267854c88a2aa23bb9dabb261033448919f3bf590a636fa88cfd3a5b58cb6\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_requests_test.go\",\"bytes\":7329,\"lines\":205,\"blob\":\"e789fb0fe27471106ee509b65867a6190229d8d5\",\"sha256\":\"7bbb33bf545f6563cf5eefcc36d1d7fb046a6a1f6ad6c0d858e6b42761d6b697\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_routes_test.go\",\"bytes\":3168,\"lines\":111,\"blob\":\"f635cfb15f244fe69c8797c7eb0e0342979929f8\",\"sha256\":\"ee742a4ac5decc9d60e60657356560528cc961b957ad4c0dffc46e5058aac434\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_routing_test.go\",\"bytes\":6305,\"lines\":189,\"blob\":\"44063dde743dd1a43c9b39987388a112bb21ff8d\",\"sha256\":\"df68268c7c7819fce2338590d1c88350285bcd6adf64c3ecd12b18aa6e44dfce\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_target_test.go\",\"bytes\":4621,\"lines\":170,\"blob\":\"839aa6185bda860cf48a38e99b657b9277742226\",\"sha256\":\"6e996fb624a949dc12fd79d211d3bc38651c635de0d2ab37a1c6e7a01710eea8\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_transport_test.go\",\"bytes\":5001,\"lines\":169,\"blob\":\"8f090a9f9606bb6fb04092854971b3d542445271\",\"sha256\":\"503f6c1d469dacb67d7eb92df0cf16fabd5b12f07c7c6be7839a6e08edecb4dd\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_witness_test.go\",\"bytes\":8319,\"lines\":228,\"blob\":\"6bdb0fa8f2c0e4905ef3911f3139eb9571c03988\",\"sha256\":\"b6c9bdcfa25152fc6b2e2a887e0020955adf11ce4e437978016d07f0e33e615a\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-restore-proof-reporter.ts\",\"bytes\":11344,\"lines\":178,\"blob\":\"50434a8597d3015182348bd9bdef21a7a05eef55\",\"sha256\":\"6cf05f505c6982690fc84c5eb22c43010830407df557532f939ee06235197c2a\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-restore-recovery-fixture.ts\",\"bytes\":10558,\"lines\":175,\"blob\":\"a7a05860c53393c98c752801cd9f4d9787a2c86e\",\"sha256\":\"17de23167ba32c96a437efa0663cac17f4c18a68acb80d03cdd18e0feb57eab5\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-restore-recovery-helpers.ts\",\"bytes\":16794,\"lines\":231,\"blob\":\"491c726fbcb67e429ff36748ca90f2470f1fa123\",\"sha256\":\"c43fcc6562f06889d7ad6f94801f0ca3703df2567ba00f3d9b057cbaa927becb\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-restore-recovery-network.ts\",\"bytes\":2687,\"lines\":54,\"blob\":\"9f7ddc08f035b3e80a585df55ab7657c6e6913c1\",\"sha256\":\"3071512d364cf8dde4a1681395d0945ba53560c3094514e69a5d384fb78e9d69\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-restore-recovery.config.ts\",\"bytes\":704,\"lines\":27,\"blob\":\"c0c4984307cfa2ffb37116242e4c2346ffa7eb58\",\"sha256\":\"82f6ac79869df0167c99b7006dbfefc239eb1e82c04cbd60872b03aef1484e28\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-restore-recovery.journey.ts\",\"bytes\":1648,\"lines\":28,\"blob\":\"86b4f5f1ab2505598b8236dfb022f3e0a6c1116b\",\"sha256\":\"e519904a37ac84fdfcf588ae84e62e1b3ebfb5143e6f3c81d38b79100d1b92a9\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/test-contract.json\",\"bytes\":10916,\"lines\":268,\"blob\":\"4d635e169064fadf08adf56240c72682ca2b3d1b\",\"sha256\":\"c48806bfd8595c947d82e5b55c749ca84aa80906d61dedde6071caad32a5b1ac\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture-source-manifest.json\",\"blob\":\"62bf1292673af484aa170bb0534d66677e8f250d\",\"bytes\":19363,\"lines\":208,\"sha256\":\"b627c147c2a3f569ce6df2f9122a15a9fcefdb6a348216640fc8b5cf15e5a9a7\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-save-recovery-auth.ts\",\"bytes\":2649,\"lines\":37,\"blob\":\"3bb4120bc424eba05bb8df6e9cdc703fb7609ac0\",\"sha256\":\"d45a5a79ad536a6e335b6e18f2efcfa42af9c31dc09864f2b5a0b595ea2da120\",\"getVerified\":true},{\"path\":\"apps/subtitles/e2e/subtitle-save-attachment-reader.cjs\",\"bytes\":1739,\"lines\":46,\"blob\":\"a5ff80f328abe74ec393b2df2fe5f7c4a0bc629c\",\"sha256\":\"3655771c8dca52160efc8ea7db9805896ec2232f42c43174592b3fcefdd88ef2\",\"getVerified\":true},{\"path\":\"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/canonical-adoption-source-manifest.json\",\"bytes\":14642,\"lines\":43,\"blob\":\"59b741a7350ebd9a58ef4b9350b41566fb4ad279\",\"sha256\":\"3abdd4b9b1686428f51b6a74259da4e77fb05cbb36bdc492ca9503bb0efe2926\",\"getVerified\":true}]}")

    def read_startup_package(self, value):
        import json
        from unittest.mock import patch
        import campaign_r06_restore_runtime_sources as sources
        with patch.object(sources,"read_bytes",return_value=json.dumps(value).encode()):
            return sources.package_manifest()

    def test_published_startup_raw_inputs_bind_without_qualification(self):
        value=self.startup_package()
        self.assertEqual(self.read_startup_package(value),value)
        self.assertEqual(value["currentInputBaseline"]["formatterQualification"],"pending")
        self.assertIsNone(value["currentInputBaseline"]["canonicalOutputs"])

    def test_startup_manifest_rejects_prior_canonical_pin_substitution(self):
        import copy
        prior=[{"path":"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_filesystem_test.go","bytes":1960,"sha256":"14ad292f5db92043f44cdba9f62abcdd6a2f875da9e616508aa7ae04da8db0b1","blob":"95e46d54ad38b19aae445e7886a9a10579e3e6ce"},{"path":"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_main_test.go","bytes":5447,"sha256":"9ddf2e4a5136dc0cf1f6954e324d001b78a51f0fc79459405ce357d6721e6a89","blob":"21b445622772471c1af71fb7c20cd28a1273aca2"},{"path":"apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture-source-manifest.json","bytes":17101,"sha256":"776916da74d6a165ab157f975b85e668c917552572dfec3a91ec9a7869321aa3","blob":"8a2b3396e1c372b6b5732eb034f4ae0da8016760"}]
        for stale in prior:
            value=self.startup_package()
            next(row for row in value["inputs"] if row["path"]==stale["path"]).update(stale)
            with self.assertRaises(ValueError):self.read_startup_package(value)

    def test_startup_manifest_rejects_missing_duplicate_and_wrong_raw_pins(self):
        import copy
        path="apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/restore_main_test.go"
        for change in ("missing","duplicate","bytes","sha256","blob"):
            value=self.startup_package();target=next(row for row in value["inputs"] if row["path"]==path)
            if change=="missing":value["inputs"].remove(target)
            elif change=="duplicate":value["inputs"].append(copy.deepcopy(target))
            else:target[change]=True if change=="bytes" else "0"*(64 if change=="sha256" else 40)
            with self.assertRaises(ValueError):self.read_startup_package(value)

    def test_startup_manifest_rejects_unqualified_output_or_baseline_claim(self):
        for changes in ({"commit":"3d65d3a75973122ad795e24d539425826a40182e"},
                        {"tree":"76c21f9606c95b1fbeaf34251d7c2c0191efc2ec"},
                        {"formatterQualification":"qualified"},{"canonicalOutputs":{"invented":"0"*40}},
                        {"kind":"canonical-adopted"},{"private":"fictional private"}):
            value=self.startup_package();value["currentInputBaseline"].update(changes)
            with self.assertRaises(ValueError):self.read_startup_package(value)
        value=self.startup_package();value.pop("currentInputBaseline")
        with self.assertRaises(ValueError):self.read_startup_package(value)

    def test_startup_isolation_literals_agree_and_stale_receipt_text_rejects(self):
        import sys
        import campaign_r06_restore_runtime_artifacts as artifacts
        from test_campaign_r06_restore_runtime_sources import RestoreSourceControls
        expected="Published 2b816fa8 startup-corrected raw inputs; stock formatter qualification and hosted runtime isolation proof remain pending."
        helper=RestoreSourceControls()
        if "campaign_r06_restore_runtime_entry" in sys.modules:
            entry=sys.modules["campaign_r06_restore_runtime_entry"]
        else:
            import runpy,types
            from pathlib import Path
            entry=types.SimpleNamespace(**runpy.run_path(str(Path(__file__).with_name("campaign-r06-restore-browser.py")),run_name="restore-entry-tests"))
        self.assertEqual(artifacts.PENDING,expected)
        self.assertEqual(helper.base_receipt()["pendingIsolation"],expected)
        self.assertIn(expected,entry.main.__code__.co_consts)
        values=helper.green_bundle();artifacts.admit_bundle(artifacts.seal(values),"a"*40,"b"*40)
        for stale in ("Published3d65 baseline; separately reviewed startup settings/hardware correction remains pending.",
                      "Startup canonical outputs qualified and all runtime isolation accepted"):
            values["receipt.json"]["pendingIsolation"]=stale
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(values),"a"*40,"b"*40)
