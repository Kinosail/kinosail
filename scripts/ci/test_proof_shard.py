"""Media proof shards reject unsafe manifests, stop after failures, and never pass without evidence."""

import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import proof_shard

ROOT = Path(__file__).resolve().parents[2]
VALID = {
    "player": {"one": {"browser": False, "timeout": 5, "gomaxprocs": 2,
                       "commands": [{"argv": ["python3", "apps/player/scripts/ok.py"]}],
                       "evidence": [".verification/ok/*/receipt.json"]},
               "two": {"browser": True, "timeout": 5, "gomaxprocs": 2,
                       "commands": [{"argv": ["python3", "apps/player/scripts/ok.py"], "env": {"KINOSAIL_X": "1"}, "always": True}],
                       "evidence": [".verification/ok/*/receipt.json"]}},
}


def write(directory, manifest):
    path = Path(directory) / "proofs.json"
    path.write_text(json.dumps(manifest))
    return path


class ManifestTests(unittest.TestCase):
    def test_independent_media_proofs_run_as_isolated_shards(self):
        source = (ROOT / '.github/workflows/app.yml').read_text()
        plan = source.split('  proof-plan:\n', 1)[1].split('\n  proofs:\n', 1)[0]
        self.assertIn('run: python3 scripts/ci/proof_shard.py plan "$APP"', plan)
        proofs = source.split('  proofs:\n', 1)[1].split('\n  required:\n', 1)[0]
        self.assertIn('needs: proof-plan', proofs)
        self.assertIn("if: inputs.app == 'player' && fromJSON(inputs.plan)[inputs.app]", proofs)
        self.assertIn('fail-fast: false', proofs)
        self.assertIn('include: ${{ fromJSON(needs.proof-plan.outputs.shards) }}', proofs)
        self.assertIn('timeout-minutes: ${{ matrix.timeout }}', proofs)
        self.assertIn('run: python3 scripts/ci/proof_shard.py run "$APP" "$SHARD" --evidence-dir "$RUNNER_TEMP/proof-evidence"', proofs)
        self.assertNotIn('continue-on-error', proofs)
        self.assertNotIn('docker/', proofs)
        upload = proofs.split('      - name: Keep safe media proof evidence\n', 1)[1]
        self.assertIn('if: always()', upload)
        self.assertIn('if-no-files-found: error', upload)
        self.assertIn('name: ${{ inputs.app }}-proof-${{ matrix.shard }}-evidence', upload)
        startup = proof_shard.load()['player']['startup']
        scripts = [command['argv'][1] for command in startup['commands']]
        self.assertEqual(scripts, ['apps/player/scripts/test-hls-source-version.py', 'apps/player/scripts/test-startup-local.py',
                                   'apps/player/scripts/hls-phase-evidence.py'])
        self.assertEqual([command['always'] for command in startup['commands']], [False, False, True])

    def test_checked_in_manifest_plans_only_player_media_proofs(self):
        manifest = proof_shard.load()
        shards = proof_shard.plan("player", manifest)
        self.assertTrue(shards)
        self.assertEqual({tuple(shard) for shard in shards}, {("shard", "browser", "timeout")})
        with self.assertRaises(ValueError):
            proof_shard.plan("subtitles", manifest)

    def test_malformed_manifests_are_rejected(self):
        broken = []
        for mutate in (
            lambda m: m.update(subtitles=m.pop("player")),
            lambda m: m.update(extra={}),
            lambda m: m["player"].update({"Bad_Name": m["player"]["one"]}),
            lambda m: m["player"]["one"].pop("evidence"),
            lambda m: m["player"]["one"].update(timeout=0),
            lambda m: m["player"]["one"].update(gomaxprocs=99),
            lambda m: m["player"]["one"].update(browser="yes"),
            lambda m: m["player"]["one"]["commands"][0].update(argv=["bash", "-c", "true"]),
            lambda m: m["player"]["one"]["commands"][0].update(argv=["python3", "../outside.py"]),
            lambda m: m["player"]["one"]["commands"][0].update(argv=["python3", "apps/player/scripts/../../x.py"]),
            lambda m: m["player"]["one"]["commands"][0].update(env={"PATH": "/tmp"}),
            lambda m: m["player"]["one"]["commands"][0].update(always="true"),
            lambda m: m["player"]["one"]["commands"][0].update(shell="x"),
            lambda m: m["player"]["one"].update(evidence=["/etc/passwd"]),
            lambda m: m["player"]["one"].update(evidence=[".verification/../secrets/*"]),
            lambda m: m["player"]["one"].update(evidence=[]),
            lambda m: m["player"]["one"].update(commands=[]),
        ):
            manifest = copy.deepcopy(VALID)
            mutate(manifest)
            broken.append(manifest)
        with tempfile.TemporaryDirectory() as directory:
            proof_shard.load(write(directory, VALID))
            for manifest in broken:
                with self.subTest(manifest=json.dumps(manifest)[:80]), self.assertRaises(ValueError):
                    proof_shard.load(write(directory, manifest))
            with self.assertRaises(ValueError):
                proof_shard.plan("retired", proof_shard.load(write(directory, VALID)))


class RunTests(unittest.TestCase):
    def fake_root(self, directory):
        root = Path(directory) / "repo"
        scripts = root / "apps/player/scripts"
        scripts.mkdir(parents=True)
        (scripts / "ok.py").write_text(
            "import os, pathlib, sys\n"
            "run = pathlib.Path('.verification/ok/1'); run.mkdir(parents=True, exist_ok=True)\n"
            "(run / 'receipt.json').write_text(os.environ.get('KINOSAIL_X', '') + os.environ['GOMAXPROCS'])\n"
            "pathlib.Path(os.environ['TEST_PROOF_LOG']).open('a').write(sys.argv[1] + '\\n')\n")
        (scripts / "fail.py").write_text("import os, pathlib, sys\n"
            "pathlib.Path(os.environ['TEST_PROOF_LOG']).open('a').write(sys.argv[1] + '\\n'); sys.exit(3)\n")
        return root

    def run_shard(self, directory, manifest, app, shard):
        root = self.fake_root(directory)
        log = Path(directory) / "log"
        log.write_text("")
        evidence = Path(directory) / "evidence"
        with mock.patch.object(proof_shard, "ROOT", root), mock.patch.dict(os.environ, {"TEST_PROOF_LOG": str(log)}):
            passed = proof_shard.run(app, shard, proof_shard.load(write(directory, manifest)), evidence)
        receipt = json.loads((evidence / "shard-receipt.json").read_text())
        return passed, log.read_text().split(), receipt, evidence

    def test_failure_skips_later_commands_unless_always_and_keeps_evidence(self):
        manifest = copy.deepcopy(VALID)
        manifest["player"]["one"]["commands"] = [
            {"argv": ["python3", "apps/player/scripts/ok.py", "first"]},
            {"argv": ["python3", "apps/player/scripts/fail.py", "second"]},
            {"argv": ["python3", "apps/player/scripts/ok.py", "third"]},
            {"argv": ["python3", "apps/player/scripts/ok.py", "fourth"], "always": True},
        ]
        with tempfile.TemporaryDirectory() as directory:
            passed, ran, receipt, evidence = self.run_shard(directory, manifest, "player", "one")
            self.assertEqual((evidence / ".verification/ok/1/receipt.json").read_text(), "2")
        self.assertFalse(passed)
        self.assertEqual(ran, ["first", "second", "fourth"])
        self.assertEqual([c["status"] for c in receipt["commands"]], ["passed", "failed", "skipped", "passed"])
        self.assertEqual(receipt["commands"][1]["exitCode"], 3)
        self.assertEqual(receipt["result"], "failed")

    def test_passing_shard_applies_bounded_environment_and_requires_evidence(self):
        manifest = copy.deepcopy(VALID)
        manifest["player"]["two"]["commands"][0]["argv"].append("only")
        with tempfile.TemporaryDirectory() as directory:
            passed, ran, receipt, evidence = self.run_shard(directory, manifest, "player", "two")
            self.assertTrue(passed)
            self.assertEqual(ran, ["only"])
            self.assertEqual((evidence / ".verification/ok/1/receipt.json").read_text(), "12")
            self.assertEqual(receipt["commands"][0]["env"], ["KINOSAIL_X"])
            self.assertNotIn("1", json.dumps(receipt["commands"][0]["env"]))
        manifest["player"]["two"]["evidence"] = [".verification/missing/*/receipt.json"]
        with tempfile.TemporaryDirectory() as directory, self.assertRaises(ValueError):
            self.run_shard(directory, manifest, "player", "two")

    def test_unknown_shard_has_no_side_effects(self):
        with tempfile.TemporaryDirectory() as directory:
            root = self.fake_root(directory)
            evidence = Path(directory) / "evidence"
            with mock.patch.object(proof_shard, "ROOT", root), self.assertRaises(ValueError):
                proof_shard.run("player", "missing", proof_shard.load(write(directory, VALID)), evidence)
            self.assertFalse(evidence.exists())
            self.assertFalse((root / ".verification").exists())


if __name__ == "__main__":
    unittest.main()
