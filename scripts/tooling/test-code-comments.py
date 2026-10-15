#!/usr/bin/env python3

import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

CHECKER = Path(__file__).with_name('check-code-comments.py')


class CodeCommentsCommandTests(unittest.TestCase):
    def test_rejection_and_program_data_at_command_boundary(self):
        clean = {
            'main.go': 'package main\nvar url = "https://example.invalid"\n',
            'main.ts': 'const pattern = /https?:\\/\\//; const text = `// data`;\n',
            'main.swift': 'let text = #"https://example.invalid/*data*/"#\n',
            'main.kt': 'val text = """// data"""\n',
            'main.py': '"""Runtime documentation."""\ntext = "# data"\n',
            'main.sh': '#!/bin/sh\ncat <<\'DATA\'\n# data\nDATA\n',
            'main.yaml': 'text: |\n  # data\n',
            'main.html': '<script>const value = "// data";</script><style>.x{color:red}</style>\n',
            'main.css': '.x::before { content: "/* data */"; }\n',
            'Makefile': 'all:\n\tprintf "%s" "# data"\n',
            'Containerfile': 'FROM scratch\nRUN echo "# data"\nLABEL value="# data"\n',
            'Containerfile.test': 'FROM scratch\nRUN echo "# data"\n',
            'workflow.yaml': 'run: |\n  cat <<\'DATA\'\n  # data\n  DATA\n  echo "# data"\n',
            'Main.java': 'class Main { String value = "https://example.invalid"; }\n',
            'fixture.d.cts': 'declare const value: "// data";\n',
            'impeccable': '#!/bin/sh\necho "# data"\n',
            'launcher.cmd': '@echo off\necho "REM data"\n',
            '.env.example': 'VALUE="https://example.invalid/#data"\n',
            'gradle.properties': 'value=https://example.invalid/#data\n',
        }
        rejected = {
            'main.go': '// explanation\n',
            'main.ts': 'const value = 1; // explanation\n',
            'main.swift': '/* outer /* nested */ explanation */\n',
            'main.kt': '/** Documentation comment. */\n',
            'main.py': 'text = 1 # explanation\n',
            'main.sh': '# explanation\n',
            'main.yaml': 'text: value # explanation\n',
            'main.html': '<script>/* explanation */</script><style>/* explanation */</style>\n',
            'main.css': '/* explanation */\n',
            'Makefile': 'other:\n\techo value # explanation\n',
            'Containerfile': 'RUN echo value # explanation\n',
            'Containerfile.test': '# explanation\n',
            'workflow.yaml': 'run: |\n  echo value # explanation\n',
            'Main.java': '// explanation\n',
            'fixture.d.cts': '// explanation\n',
            'impeccable': '# explanation\n',
            'launcher.cmd': 'rem explanation\n',
            '.env.example': '# explanation\n',
            'gradle.properties': '# explanation\n',
        }
        with tempfile.TemporaryDirectory() as directory:
            repo = Path(directory)
            subprocess.run(['git', 'init', '-q', directory], check=True)
            for name, content in clean.items():
                (repo / name).write_text(content)
            self.run_check(repo, 0)
            for name, content in rejected.items():
                with self.subTest(name=name):
                    (repo / name).write_text(clean[name] + content)
                    result = self.run_check(repo, 1)
                    self.assertIn(name + ':', result.stderr)
                    (repo / name).write_text(clean[name])
            (repo / 'main.go').write_text('//go:build linux\n\npackage main\n//go:embed asset.txt\nvar asset string\nvar fixture = 0 // #nosec G101 -- Synthetic fixture data.\n')
            (repo / 'main.sh').write_text('#!/bin/sh\n# shellcheck disable=SC2016\necho ok\n')
            (repo / 'main.ts').write_text('/*! SPDX-License-Identifier: MIT */\nconst value = 1;\n')
            for name in ('third_party/vendor.js', 'engineering/qa/snapshot.py'):
                path = repo / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text('// archived or vendored data\n')
            self.run_check(repo, 0)

    def run_check(self, repo, expected):
        def state():
            return {str(p.relative_to(repo)): hashlib.sha256(p.read_bytes()).hexdigest()
                    for p in repo.rglob('*') if p.is_file() and '.git' not in p.parts}
        before = state()
        result = subprocess.run([sys.executable, str(CHECKER)], cwd=repo,
                                capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        self.assertEqual(state(), before)
        return result


if __name__ == '__main__':
    unittest.main()
