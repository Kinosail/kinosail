# Making Codex produce product-specific UI reliably

Research snapshot: 2026-08-25.

This note answers a narrow question: how should Kinosail configure Codex so frontend work consistently follows the product's design system instead of falling back to generic AI-generated UI? It uses only first-party OpenAI documentation. It does not recommend hosted design services, remote design assets, third-party MCP connections, or a new frontend dependency.

## Executive conclusion

There is no single phrase that can guarantee good design. The reliable approach is a small, layered control loop:

1. `AGENTS.md` owns durable repository rules and points Codex to the authoritative design contract.
2. A focused repo skill owns the repeatable UI workflow and finish gate.
3. Each task prompt states the user-visible outcome and acceptance evidence, without duplicating the whole design manual.
4. Codex renders the real app with representative data, inspects screenshots and interactions at multiple viewports, critiques concrete weaknesses, revises, and reruns the checks.
5. A small set of representative UI tasks is rerun when the instructions or skill change.

This follows OpenAI's guidance in three important ways. Codex loads `AGENTS.md` before work and layers global, repository, and nested guidance by scope; skills are progressively disclosed and trigger from concise descriptions; and current prompting guidance recommends outcome-first prompts, product/domain context, explicit success criteria, rendered inspection, and representative evaluations rather than increasingly long instruction stacks. ([AGENTS.md guidance](https://learn.chatgpt.com/docs/agent-configuration/agents-md); [skills guidance](https://learn.chatgpt.com/docs/build-skills); [model guidance](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.5))

## What the official frontend guidance changes

OpenAI's current frontend prompt instructions are unusually specific. They direct the model to understand the audience and domain, follow an existing design framework, make the actual experience the first screen, build expected states and controls, and avoid familiar generated-UI defaults such as decorative card-heavy composition, nested cards, gratuitous gradients, bloated hero treatment, unstable responsive geometry, and explanatory text that should not be part of the product. They also require Playwright screenshot checks across desktop and mobile for framing, overlap, interactivity, and asset rendering. ([Frontend prompt instructions](https://developers.openai.com/api/docs/guides/frontend-prompt))

The relevant translation for Kinosail is:

- Start from a private media-server task, not a component catalog or generic dashboard archetype.
- Treat `DESIGN.md`, existing shared templates, role tokens, populated library data, and current browser behavior as the design context.
- Decide what the user must notice and do before choosing containers or components.
- Build the real browse, playback, settings, or authentication surface, including loading, empty, error, disabled, success, and recovery states that the workflow can reach.
- Validate stable media ratios, toolbar and grid geometry, long labels, focus, missing artwork, and action hierarchy at the actual supported widths.
- Inspect rendered output and revise it. Passing compilation, unit tests, axe, or a source heuristic alone is not visual evidence.

The official prompt suggests using an existing icon library rather than drawing inconsistent icons. That does **not** require Kinosail to add Lucide or any other dependency. Because Kinosail's product contract forbids new remote design dependencies and already supports local embedded assets, the product-specific adaptation is one coherent local icon set using the existing shared asset/template seam. This is an explicit project constraint applied to the broader OpenAI principle of icon consistency.

## Put each instruction in one place

OpenAI recommends lean prompts: state each instruction once, keep examples and style guidance only when they encode a product requirement or correct a measured failure, expose only relevant tools, and compare changes on representative tasks. Repeated or over-specified instructions can narrow the model's search space and reduce quality. ([Model guidance](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.5); [current model guidance](https://developers.openai.com/api/docs/guides/latest-model))

Kinosail should therefore keep the layers deliberately different:

| Layer | Owns | Should not duplicate |
| --- | --- | --- |
| `DESIGN.md` | Product character, tokens, composition, component behavior, responsive modes, accessibility, and definition of done | Agent mechanics and task-specific steps |
| Root `AGENTS.md` | Durable requirement to read the design contract, use the local UI skill, render/inspect real states, and report evidence | Token tables, full anti-pattern catalogs, or page-by-page design recipes |
| `anti-ai-slop-ui/SKILL.md` | The repeatable preflight, implementation, browser inspection, critique, scoring, and stop conditions | A second copy of the full design contract |
| Task prompt | The concrete user outcome, scope, affected flow, success criteria, and any supplied reference | Repository-wide rules already loaded from `AGENTS.md` |

Codex concatenates applicable `AGENTS.md` files from broad to specific scope, with nearer files taking precedence, and stops at a combined size limit that defaults to 32 KiB. That makes concise root guidance plus a focused skill more robust than copying a large design manifesto into multiple instruction files. ([AGENTS.md discovery and precedence](https://learn.chatgpt.com/docs/agent-configuration/agents-md))

## Recommended `AGENTS.md` contract

The persistent rule should be short and verifiable. The existing Kinosail frontend section is already close; its essential contract is:

- Read `DESIGN.md` before changing visible UI and preserve the shared tokens and component seams.
- Invoke the repo's local `anti-ai-slop-ui` skill for any visible UI generation, redesign, or review.
- Use real product data and the real user workflow; do not begin from a generic dashboard, landing-page, or component-gallery template.
- Do not add hosted design services, remote design assets, third-party design MCP connections, or a new UI dependency for this workflow.
- Render meaningful changes in a browser at desktop, tablet, and compact mobile sizes; exercise relevant conditional states and interactions; run the local slop, geometry, screenshot, and accessibility checks.
- Perform at least one evidence-based critique-and-revision pass before completion, then report exactly what was inspected and what remains unverified.

This belongs in `AGENTS.md` because OpenAI documents it as the persistent source Codex reads before work. The detailed procedure belongs in the skill so it loads only for matching frontend tasks. ([AGENTS.md guidance](https://learn.chatgpt.com/docs/agent-configuration/agents-md); [skill progressive disclosure](https://learn.chatgpt.com/docs/build-skills))

## Recommended skill shape

OpenAI says skill selection depends on the `description`, recommends front-loading clear trigger words and boundaries, keeping each skill focused on one job, writing imperative steps with explicit inputs and outputs, and testing prompts against the description. ([Build skills: activation and best practices](https://learn.chatgpt.com/docs/build-skills))

For Kinosail, the skill should be repo-specific and use this sequence:

1. **Trigger precisely.** The description should say it applies to any Kinosail-visible HTML, CSS, browser JavaScript, asset, interaction-copy, responsive, accessibility, or visual-review task, and should explicitly exclude backend-only work.
2. **Load local truth.** Read `DESIGN.md`, the target templates/styles/scripts, the nearest existing component, tests, and one populated rendered reference. Do not contact a hosted design service.
3. **Write a five-line design brief.** State the user goal, first/second/third hierarchy, composition, interaction and recovery model, and Kinosail visual character. Concrete decisions such as “compact owner utility with 14px dense controls” are useful; labels such as “modern” are not.
4. **Inventory the reachable state set.** Mark which default, hover, focus, active, selected, disabled, loading, success, empty, error, permission, and recovery states apply. Include long text, missing artwork, and populated data.
5. **Reuse before inventing.** Search existing shared templates, CSS roles, icons, and test helpers. Add a new primitive only when the current system has no suitable role.
6. **Implement the smallest coherent change.** Preserve the Go + HTMX architecture and local asset boundary.
7. **Render the exact changed flow.** Inspect the populated app near 1440x900, 1024x768, and 390x844; include affected conditional states. Check first-frame hierarchy, clipping, overlap, wrapping, scroll, media ratios, focus, and console errors.
8. **Run objective checks.** Use focused Go/API/web tests, axe, geometry assertions, screenshots, the local slop heuristic, and the applicable state tests. Objective checks find regressions but do not replace visual judgment.
9. **Critique and revise.** Name at least three concrete weaknesses or explicitly record why fewer exist; fix every material finding; render and inspect again.
10. **Stop only on evidence.** Completion requires the intended workflow to work, the relevant checks to pass, and the final report to identify any browser, device, state, fixture, or performance boundary.

OpenAI's model guidance specifically advises rendering visual artifacts, checking layout, clipping, spacing, missing content, and consistency, then revising until the render matches the requirements. Its frontend instructions add Playwright screenshots and responsive geometry checks. These should be mandatory skill steps rather than optional suggestions. ([Model guidance: check the work](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.5); [frontend verification guidance](https://developers.openai.com/api/docs/guides/frontend-prompt))

## Use outcome-first task prompts

The task prompt should describe the destination, not restate the implementation workflow. A good Kinosail prompt has four short parts:

```text
Goal: [user-visible outcome in the real Kinosail flow]

Context: [affected route/surface, user role, real data or supplied reference]

Success means:
- [functional behavior]
- [product-specific hierarchy or interaction result]
- [required states and responsive widths]
- [evidence to run and report]

Constraints: preserve DESIGN.md and existing shared seams; no hosted design
service, remote asset, third-party design connection, or unrelated redesign.
```

OpenAI recommends this outcome-first structure because the model performs better when given the goal, constraints, evidence rules, success criteria, and stopping rules while retaining freedom to choose the efficient implementation path. Absolute rules should be reserved for true invariants; UI judgment should use explicit decision rules and acceptance evidence. ([Model prompting guidance](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.5))

## Add a repeatable UI eval set

Instruction changes should be tested instead of judged from one attractive screen. OpenAI recommends tuning prompts and reasoning against representative examples, changing one instruction group at a time, and rerunning the same evaluations. The skills documentation also recommends testing prompts against the skill description to verify trigger behavior. ([Current model guidance](https://developers.openai.com/api/docs/guides/latest-model); [skills best practices](https://learn.chatgpt.com/docs/build-skills))

Kinosail's local eval set should contain at least these five tasks:

1. A populated Movies browse page with real posters, long titles, missing art, filtering, and mobile two-column layout.
2. A media detail/player flow covering play or resume, tracks, loading, direct/compatibility state, and recoverable failure.
3. A dense Owner settings form covering disabled/external values, validation error, saving, restart consequence, and narrow layout.
4. An authentication/onboarding flow covering default, MFA/passkey, error, and recovery states.
5. An empty/permission-limited library or search flow with one honest recovery action.

Score the same artifacts after every material skill change:

- product/domain specificity and correct first-screen hierarchy;
- reuse of Kinosail tokens, layout, icons, and component seams;
- absence of generic hero, nested-card, decorative-gradient, fake-metric, and over-rounded defaults;
- completeness of expected and failure/recovery states;
- desktop, tablet, mobile, keyboard, and accessibility behavior;
- screenshot geometry and populated-data quality; and
- whether the critique pass found and corrected material defects.

Store screenshots and machine-readable results locally. A heuristic slop score is useful as one signal, but the pass condition must include rendered browser evidence and human-readable critique because source patterns cannot establish visual hierarchy or composition.

## Model and reasoning settings are secondary

OpenAI's current guidance describes GPT-5.6 as stronger at frontend layout, visual hierarchy, and design judgment. It also warns that higher reasoning effort is not automatically better and recommends comparing settings on representative tasks before paying the latency and cost. ([GPT-5.6 model guidance](https://developers.openai.com/api/docs/guides/latest-model))

Therefore:

- Use the strongest available Codex model for broad redesigns or hard visual diagnosis when quality matters more than latency.
- Use faster models for small, well-bounded iterations only after the same UI evals show they preserve quality.
- Do not expect a model switch or higher reasoning setting to compensate for missing product context, absent browser evidence, or contradictory instructions.
- Keep the skill and eval set stable while comparing model or reasoning settings, so the comparison measures the model rather than simultaneous prompt drift.

## What not to add

- No hosted design generator, Figma connection, third-party MCP server, remote font/icon/image dependency, or telemetry service is necessary to enforce this workflow.
- Do not duplicate the entire official frontend prompt in `AGENTS.md`, `DESIGN.md`, and the skill. Translate its principles once into Kinosail's existing ownership layers.
- Do not use a list of banned aesthetics as the whole design system. Pair anti-patterns with the product goal, hierarchy, composition, state model, and real data.
- Do not certify design quality from a source linter, axe, compilation, or a single empty-state screenshot.
- Do not increase reasoning effort or prompt length by default. Measure changes against the same representative UI tasks.

## Primary sources

- [OpenAI, Frontend prompt instructions](https://developers.openai.com/api/docs/guides/frontend-prompt)
- [OpenAI, Custom instructions with AGENTS.md](https://learn.chatgpt.com/docs/agent-configuration/agents-md)
- [OpenAI, Build skills](https://learn.chatgpt.com/docs/build-skills)
- [OpenAI, GPT-5.5 model and prompting guidance](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-5.5)
- [OpenAI, current model guidance](https://developers.openai.com/api/docs/guides/latest-model)
