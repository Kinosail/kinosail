"""Bounded lexical preservation for the exact reviewed Restore Go inputs.

This is not a Go compiler or a general parser. Unrecognized syntax is blocked.
Only import-spec order, Go semicolons and recognized list-final commas normalize.
Comments, literal spelling, bindings and declaration order remain exact.
"""
import re

WORDS = re.compile(r"[A-Za-z_][A-Za-z_0-9]*")
NUMBER = re.compile(r"(?:0[xX][0-9A-Fa-f_]+(?:\.[0-9A-Fa-f_]*)?(?:[pP][+-]?[0-9_]+)?|"
                    r"0[bB][01_]+|0[oO][0-7_]+|"
                    r"(?:[0-9][0-9_]*(?:\.[0-9_]*)?|\.[0-9_]+)(?:[eE][+-]?[0-9_]+)?)(?:i)?")
KEYWORDS = frozenset(("break case chan const continue default defer else fallthrough for func go "
                      "goto if import interface map package range return select struct switch type var").split())
ENDS = frozenset(("break", "continue", "fallthrough", "return", "++", "--", ")", "]", "}"))
BUILTIN_TYPES = frozenset(("bool byte complex64 complex128 error float32 float64 int int8 int16 int32 "
                           "int64 rune string uint uint8 uint16 uint32 uint64 uintptr any").split())
OWNED_TYPES = frozenset(("deniedTransport restoreCapture restoreCheckedListener restoreControlResponse "
                         "restoreExchange restoreInspection restorePublicReceipt restoreRig restoreSetupInput "
                         "restoreSnapshot restoreTarget URL").split())
OPERATORS = tuple(sorted(("<<= >>= &^= ... ++ -- == != <= >= := <- && || += -= *= /= %= &= |= ^= "
                          "<< >> &^ + - * / % & | ^ < > = ! ( ) [ ] { } , . ; : ~").split(),
                         key=len, reverse=True))


def ends_statement(token):
    kind, value = token
    return kind in ("number", "string", "rune") or value in ENDS or kind == "word" and value not in KEYWORDS


def quoted(source, offset):
    quote = source[offset]
    end = offset + 1
    while end < len(source):
        char = source[end]
        if char == quote:
            return source[offset:end + 1], end + 1
        if quote != "`" and char in "\r\n":
            raise ValueError("go-quoted-newline")
        if quote != "`" and char == "\\":
            end += 1
            if end >= len(source) or source[end] in "\r\n":
                raise ValueError("go-quoted-escape")
        end += 1
    raise ValueError("go-unclosed-literal")


def lex(data):
    if type(data) is not bytes or not data or b"\0" in data:
        raise ValueError("go-input")
    source = data.decode("utf-8")
    result, previous, offset = [], None, 0

    def newline():
        nonlocal previous
        if previous is not None and ends_statement(previous):
            result.append(("op", ";"))
            previous = ("op", ";")

    while offset < len(source):
        char = source[offset]
        if char in " \t\r\n":
            if char == "\n":
                newline()
            offset += 1
            continue
        if source.startswith("//", offset):
            end = source.find("\n", offset)
            if end < 0:
                end = len(source)
            result.append(("comment", source[offset:end]))
            offset = end
            continue
        if source.startswith("/*", offset):
            end = source.find("*/", offset + 2)
            if end < 0:
                raise ValueError("go-unclosed-comment")
            comment = source[offset:end + 2]
            result.append(("comment", comment))
            if "\n" in comment:
                newline()
            offset = end + 2
            continue
        if char in "\"'`":
            value, offset = quoted(source, offset)
            token = ("rune" if char == "'" else "string", value)
        else:
            match = WORDS.match(source, offset) or NUMBER.match(source, offset)
            if match is not None:
                value = match.group()
                token = ("word" if WORDS.fullmatch(value) else "number", value)
                offset = match.end()
            else:
                value = next((value for value in OPERATORS if source.startswith(value, offset)), None)
                if value is None:
                    raise ValueError("go-unrecognized-token")
                token = ("op", value)
                offset += len(value)
        result.append(token)
        previous = token
    newline()
    return result


def import_spec(tokens, offset):
    start = offset
    if tokens[offset][0] == "word" or tokens[offset] == ("op", "."):
        offset += 1
    if offset >= len(tokens) or tokens[offset][0] != "string":
        raise ValueError("go-import-spec")
    offset += 1
    return tuple(tokens[start:offset]), offset


def imports(tokens):
    output, offset = [], 0
    while offset < len(tokens):
        if tokens[offset] != ("word", "import"):
            output.append(tokens[offset])
            offset += 1
            continue
        offset += 1
        specs = []
        grouped = offset < len(tokens) and tokens[offset] == ("op", "(")
        if grouped:
            offset += 1
            while offset < len(tokens) and tokens[offset] != ("op", ")"):
                if tokens[offset] == ("op", ";"):
                    offset += 1
                    continue
                spec, offset = import_spec(tokens, offset)
                specs.append(spec)
                if offset < len(tokens) and tokens[offset] not in (("op", ";"), ("op", ")")):
                    raise ValueError("go-import-separator")
            if offset >= len(tokens):
                raise ValueError("go-import-close")
            offset += 1
        else:
            spec, offset = import_spec(tokens, offset)
            specs.append(spec)
        if not specs or len(specs) != len(set(specs)):
            raise ValueError("go-import-membership")
        output.append(("imports", tuple(sorted(specs))))
    return output


def brace_kind(previous, types, closed, parent):
    if previous == ("word", "struct") or previous == ("word", "interface"):
        return "type"
    if previous and previous[0] == "word" and previous[1] in types:
        return "list"
    if previous == ("op", "}") and closed == "type":
        return "list"
    if previous in (("op", "{"), ("op", ",")) and parent == "list":
        return "list"
    return "block"



def qualified_config(output):
    significant = []
    for token in reversed(output):
        if token[0] != "comment":
            significant.append(token)
            if len(significant) == 4:
                break
    return (significant[:3] == [("word", "Config"), ("op", "."), ("word", "server")]
            and (len(significant) < 4 or significant[3] != ("op", ".")))

def normalize(tokens):
    tokens = imports(tokens)
    types = set(BUILTIN_TYPES | OWNED_TYPES)
    for index, token in enumerate(tokens[:-1]):
        if token == ("word", "type") and tokens[index + 1][0] == "word":
            types.add(tokens[index + 1][1])
    output, stack = [], []
    last_closed = None
    for index, token in enumerate(tokens):
        kind, value = token
        previous = next((entry for entry in reversed(output) if entry[0] != "comment"), None)
        if kind == "op" and value in ("(", "[", "{"):
            if value == "{":
                frame = ("list" if qualified_config(output) else
                         brace_kind(previous, types, last_closed, stack[-1][1] if stack else None))
            elif value == "(":
                frame = "list" if previous and (previous[0] == "word" and
                        previous[1] not in KEYWORDS or previous == ("word", "func") or
                        previous in (("op", ")"), ("op", "]"))) else "group"
            else:
                frame = "index"
            stack.append((value, frame))
        elif kind == "op" and value in (")", "]", "}"):
            if not stack or stack[-1][0] != {")": "(", "]": "[", "}": "{"}[value]:
                raise ValueError("go-delimiters")
            _, last_closed = stack.pop()
            if output and output[-1] == ("op", ",") and last_closed == "list" and value in (")", "}"):
                output.pop()
        if token == ("op", ";") and index + 1 < len(tokens) and tokens[index + 1] in (("op", ")"), ("op", "}")):
            continue
        output.append(token)
    if stack:
        raise ValueError("go-delimiters")
    return tuple(output)


def equivalent(original, formatted):
    try:
        return normalize(lex(original)) == normalize(lex(formatted))
    except (ValueError, TypeError, UnicodeDecodeError):
        return False
