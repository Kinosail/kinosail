#!/usr/bin/env python3

from functools import lru_cache
from bisect import bisect_right
from pathlib import Path
import re
import subprocess
import sys

from tree_sitter_language_pack import get_parser

LANGUAGES = {
    '.go': 'go', '.py': 'python', '.swift': 'swift', '.kt': 'kotlin', '.kts': 'kotlin',
    '.js': 'javascript', '.mjs': 'javascript', '.cjs': 'javascript', '.ts': 'typescript',
    '.tsx': 'tsx', '.cts': 'typescript', '.java': 'java', '.cmd': 'batch',
    '.css': 'css', '.html': 'html', '.sh': 'bash', '.yaml': 'yaml',
    '.yml': 'yaml', '.toml': 'toml', '.xml': 'xml', '.plist': 'xml', '.svg': 'xml',
    '.properties': 'properties',
}
BUILD_FILES = {'Makefile': 'make', 'Containerfile': 'dockerfile', 'Dockerfile': 'dockerfile',
               'Brewfile': 'ruby', 'Gemfile': 'ruby', 'impeccable': 'bash', '.env.example': 'bash'}
VENDORED = {
    f'apps/{app}/internal/server/static/{asset}.min.js'
    for app in ('player', 'subtitles') for asset in ('hls', 'htmx')
} | {'apps/player/apps/android/gradlew', 'apps/player/apps/android/gradlew.bat'}
COMMENT_TYPES = {'comment', 'line_comment', 'block_comment', 'multiline_comment'}
DIRECTIVE = re.compile(
    rb'(?://(?:go:|line | \+build |nolint\b|lint:|swift-tools-version:|swiftlint:)|'
    rb'(?://|/\*)\s*(?:eslint\b|global[s]?\b|@ts-(?:check|nocheck|ignore|expect-error)\b|'
    rb'[#@]\s*source(?:Mapping)?URL=|[#@]__PURE__|#nosec\b)|'
    rb'// Code generated [^\n]+ DO NOT EDIT\.|'
    rb'#\s*(?:shellcheck\b|(?:type|noqa|fmt|pragma|renovate):)|'
    rb'#\s*(?:syntax|escape|check)=)'
)
LEGAL = re.compile(rb'(?:SPDX-License-Identifier:|Copyright\s+(?:\(c\)\s*)?\d{4}|'
                   rb'Licensed under|Permission is hereby granted|@license\b)', re.I)
parser_for = lru_cache(maxsize=None)(get_parser)


def language(path):
    if ({'third_party', 'vendor', 'node_modules'} & set(path.parts)
            or 'engineering/qa/' in path.as_posix() or path.as_posix() in VENDORED):
        return None
    return BUILD_FILES.get(path.name, LANGUAGES.get(path.suffix))


def comment_spans(source, lang, offset=0):
    if lang == 'batch':
        for match in re.finditer(rb'(?im)^[ \t]*@?(?:rem(?:[ \t][^\r\n]*)?|::[^\r\n]*)\r?$', source):
            yield offset + match.start(), offset + match.end()
        return
    if lang == 'dockerfile':
        yield from docker_spans(source, offset)
        return
    stack = [parser_for(lang).parse(source).root_node]
    while stack:
        node = stack.pop()
        if node.type.lower() in COMMENT_TYPES:
            yield offset + node.start_byte, offset + node.end_byte
        elif lang == 'html' and node.type == 'raw_text':
            nested = {'script_element': 'javascript', 'style_element': 'css'}.get(node.parent.type)
            if nested:
                yield from comment_spans(source[node.start_byte:node.end_byte], nested,
                                         offset + node.start_byte)
        elif lang == 'make' and node.type == 'shell_text':
            yield from comment_spans(source[node.start_byte:node.end_byte], 'bash',
                                     offset + node.start_byte)
        elif lang == 'yaml' and node.type == 'block_scalar':
            pair = node.parent.parent
            key = pair.child_by_field_name('key')
            if key and source[key.start_byte:key.end_byte] == b'run':
                start = source.index(b'\n', node.start_byte, node.end_byte) + 1
                yield from shell_block_spans(source[start:node.end_byte], offset + start)
        else:
            stack.extend(reversed(node.children))


def docker_spans(source, offset):
    spans = {(offset + match.start(), offset + match.end())
             for match in re.finditer(rb'(?m)^[ \t]*#[^\n]*', source)}
    for match in re.finditer(rb'(?im)^(?:ONBUILD[ \t]+)?(?:RUN|CMD|ENTRYPOINT)[ \t]+', source):
        end = source.find(b'\n', match.end())
        while end >= 0 and source[match.end():end].rstrip().endswith(b'\\'):
            end = source.find(b'\n', end + 1)
        if end < 0:
            end = len(source)
        command = source[match.end():end]
        if not command.lstrip().startswith(b'['):
            spans.update(comment_spans(command, 'bash', offset + match.end()))
    yield from sorted(spans)


def shell_block_spans(source, offset):
    lines = source.splitlines(keepends=True)
    indent = min((len(line) - len(line.lstrip(b' ')) for line in lines if line.strip()), default=0)
    starts, originals, chunks = [], [], []
    original = position = 0
    for line in lines:
        trim = min(indent, len(line) - len(line.lstrip(b' ')))
        starts.append(position)
        originals.append(original + trim)
        chunks.append(line[trim:])
        position += len(line) - trim
        original += len(line)
    def translate(position):
        row = bisect_right(starts, position) - 1
        return offset + originals[row] + position - starts[row]
    for start, end in comment_spans(b''.join(chunks), 'bash'):
        yield translate(start), translate(end)


def allowed(source, start, end, lang):
    raw = source[start:end]
    if start == 0 and raw.startswith(b'#!'):
        return True
    if lang == 'python' and source[:start].count(b'\n') < 2 and re.search(rb'coding[:=]\s*[-\w.]+', raw):
        return True
    if lang == 'go' and raw.startswith(b'/*') and re.match(rb'\s*import\s+"C"', source[end:]):
        return True
    return bool(DIRECTIVE.match(raw) or LEGAL.search(raw))


def main():
    if sys.argv[1:]:
        print('Usage: check-code-comments.py', file=sys.stderr)
        return 2
    try:
        root = Path(subprocess.check_output(['git', 'rev-parse', '--show-toplevel'], text=True).strip())
        names = subprocess.check_output(['git', '-C', str(root), 'ls-files', '-co',
                                         '--exclude-standard', '-z']).decode().split('\0')
        failures = checked = 0
        for name in sorted(set(filter(None, names))):
            path = Path(name)
            lang = language(path)
            actual = root / path
            if not lang or not actual.is_file() or actual.is_symlink():
                continue
            checked += 1
            source = actual.read_bytes()
            for start, end in comment_spans(source, lang):
                if not allowed(source, start, end, lang):
                    line = source[:start].count(b'\n') + 1
                    print(f'{name}:{line}: explanatory code comment is banned', file=sys.stderr)
                    failures += 1
        print(f'Code comments: {checked} files checked, {failures} forbidden comments.')
        return int(bool(failures))
    except (OSError, ValueError, LookupError, subprocess.CalledProcessError) as error:
        print(f'Code comment check failed: {type(error).__name__}', file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(main())
