# Code comment policy

Do not add explanatory comments to maintained source, tests, scripts, or build configuration. Use clear names and small functions. Put longer explanations in Markdown documentation.

Keep executable directives, required tool annotations, and legal notices. Examples include shebangs, Go build constraints and embeds, lint controls, and license notices. Keep required lint explanations with their directives. Do not change vendored dependencies, generated project metadata, or archived QA evidence to satisfy this policy. String literals, including Python docstrings and fixture contents, are program data.

Run `make comments-check` to check tracked and unignored source. This command creates an isolated Python environment under `.verification/` on first use. Required repository CI runs the same check.

## Verification contract

The command must reject standalone, inline, block, nested, and documentation comments across maintained languages. It must distinguish comments from strings, regular expressions, template literals, shell heredocs, and YAML block data. It must inspect JavaScript and CSS embedded in HTML. It must preserve executable directives and legal notices. A parser or Git failure must return a failure status. Rejection must not change source files.

The command-level regression uses a temporary Git repository and invokes the public checker. Existing app journeys cannot prove source-policy rejection or that comment-like program data remains accepted. No production test seam is needed.
