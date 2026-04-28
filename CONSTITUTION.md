# Concurrency — Constitution

> **Status:** Active. This document is the project's authoritative
> rule set. When a rule here conflicts with `CLAUDE.md`, `AGENTS.md`,
> or any guide, the Constitution wins.

## Mission

See README.md.

## Mandatory Standards

1. **Reproducibility:** every change is reproducible from a clean
   clone (`git clone <repo> && <project bootstrap>`); no hidden steps.
2. **Tests track behavior, not code:** test what the user-visible
   behavior is, not what the implementation looks like.
3. **No silent skips, no silent mocks above unit tests.**
4. **Conventional Commits** for all commits.
5. **SSH-only for git operations** (`git@…`); HTTPS prohibited.

## Numbered Rules

<!-- Rules are numbered CONST-NNN. New rules append. Removed rules
     keep their number with a "**Retired:** …" line. -->

<!-- BEGIN host-power-management addendum (CONST-033) -->

### CONST-033 — Host Power Management is Forbidden

**Status:** Mandatory. Non-negotiable. Applies to every project,
submodule, container entry point, build script, test, challenge, and
systemd unit shipped from this repository.

**Rule:** No code in this repository may invoke a host-level power-
state transition (suspend, hibernate, hybrid-sleep, suspend-then-
hibernate, poweroff, halt, reboot, kexec) on the host machine. This
includes — but is not limited to:

- `systemctl {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot,kexec}`
- `loginctl {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot}`
- `pm-{suspend,hibernate,suspend-hybrid}`
- `shutdown {-h,-r,-P,-H,now,--halt,--poweroff,--reboot}`
- DBus calls to `org.freedesktop.login1.Manager.{Suspend,Hibernate,HybridSleep,SuspendThenHibernate,PowerOff,Reboot}`
- DBus calls to `org.freedesktop.UPower.{Suspend,Hibernate,HybridSleep}`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to any value other than `'nothing'` or `'blank'`

**Why:** The host runs mission-critical parallel CLI-agent and
container workloads. On 2026-04-26 18:23:43 the host was auto-
suspended by the GDM greeter's idle policy mid-session, killing
HelixAgent and 41 dependent services. Recurring memory-pressure
SIGKILLs of `user@1000.service` (perceived as "logged out") have the
same outcome. Auto-suspend, hibernate, and any power-state transition
are unsafe for this host.

**Defence in depth (mandatory artifacts in every project):**
1. `scripts/host-power-management/install-host-suspend-guard.sh` —
   privileged installer, manual prereq, run once per host with sudo.
   Masks `sleep.target`, `suspend.target`, `hibernate.target`,
   `hybrid-sleep.target`; writes `AllowSuspend=no` drop-in; sets
   logind `IdleAction=ignore` and `HandleLidSwitch=ignore`.
2. `scripts/host-power-management/user_session_no_suspend_bootstrap.sh` —
   per-user, no-sudo defensive layer. Idempotent. Safe to source from
   `start.sh` / `setup.sh` / `bootstrap.sh`.
3. `scripts/host-power-management/check-no-suspend-calls.sh` —
   static scanner. Exits non-zero on any forbidden invocation.
4. `challenges/scripts/host_no_auto_suspend_challenge.sh` — asserts
   the running host's state matches layer-1 masking.
5. `challenges/scripts/no_suspend_calls_challenge.sh` — wraps the
   scanner as a challenge that runs in CI / `run_all_challenges.sh`.

**Enforcement:** Every project's CI / `run_all_challenges.sh`
equivalent MUST run both challenges (host state + source tree). A
violation in either channel blocks merge. Adding files to the
scanner's `EXCLUDE_PATHS` requires an explicit justification comment
identifying the non-host context.

**See also:** `docs/HOST_POWER_MANAGEMENT.md` for full background and
runbook.

<!-- END host-power-management addendum (CONST-033) -->

## Definition of Done

A change is done when:

1. The code change is committed.
2. All project-level tests pass on a clean clone.
3. All challenges in `challenges/scripts/` pass on the running host.
4. Governance docs (`CONSTITUTION.md`, `AGENTS.md`, `CLAUDE.md`) are
   coherent with the change.

## See also

- `README.md` — project overview, quickstart.
- `AGENTS.md` — guidance for AI coding agents (Codex, Cursor, etc.).
- `CLAUDE.md` — guidance specifically for Claude Code.
- `docs/HOST_POWER_MANAGEMENT.md` — CONST-033 background and runbook.


## Sixth Law — Real User Verification (Anti-Pseudo-Test Rule)

> Inherits from the root project's Anti-Bluff Testing Pact and the cross-project
> universal mandate (CONST-035). Submodule rules below are additive, never
> relaxing.

A test that passes while the feature it covers is broken for end users is the
most expensive kind of test in this codebase — it converts unknown breakage into
believed safety. This has happened in consuming projects before: tests and
Integration Challenge Tests executed green while large parts of the product
were unusable on a real device. That outcome is a constitutional failure, not a
coverage failure, and it MUST NOT recur in any module that depends on or is
depended on by this one.

Every test added MUST satisfy ALL of the following. Violation of any of them is
a release blocker, irrespective of coverage metrics, CI status, reviewer
sign-off, or schedule pressure.

1. **Same surfaces the user touches.** The test must traverse the production
   code path the user's action triggers, end to end, with no shortcut that
   bypasses real wiring.

2. **Provably falsifiable on real defects.** Before merging, the author MUST
   run the test once with the underlying feature deliberately broken (throw
   inside the function, return the wrong row, return the wrong status) and
   confirm the test fails with a clear assertion message. The PR description
   MUST state which deliberate break was used and what failure the test
   produced. A test that cannot be made to fail by breaking the thing it claims
   to verify is a bluff test by definition.

3. **Primary assertion on user-visible state.** The chief failure signal MUST
   be on something a real consumer could see or measure: rendered output,
   persisted database row, HTTP response body / status / header, file written
   to disk, packet on the wire. "Mock was invoked N times" is a permitted
   secondary assertion, never the primary one.

4. **Integration / Challenge tests are the load-bearing acceptance gate.** A
   green Challenge Test means a real consumer can complete the flow against
   real services — not "the wiring compiles". A feature for which a Challenge
   Test cannot be written is, by definition, not shippable.

5. **CI green is necessary, not sufficient.** Before any release tag is cut, a
   human (or a scripted black-box runner) MUST have exercised the feature
   end-to-end and observed the user-visible outcome.

6. **Inheritance.** This rule applies recursively to every consumer of this
   submodule. Consumer constitutions MAY add stricter rules but MUST NOT relax
   this one.


<!-- BEGIN anti-bluff-testing addendum (Article XI) -->

## Article XI — Anti-Bluff Testing (MANDATORY)

**Inherited from the umbrella project's Constitution Article XI.
Tests and Challenges that pass without exercising real end-user
behaviour are forbidden in this submodule too.**

Every test, every Challenge, every HelixQA bank entry MUST:

1. **Assert on a concrete end-user-visible outcome** — rendered DOM,
   DB rows that a real query would return, files on disk, media that
   actually plays, search results that actually contain expected
   items. Not "no error" or "200 OK".
2. **Run against the real system below the assertion.** Mocks/stubs
   are permitted ONLY in unit tests (`*_test.go` under `go test
   -short` or language equivalent). Integration / E2E / Challenge /
   HelixQA tests use real containers, real databases, real
   renderers. Unreachable real-system → skip with `SKIP-OK:
   #<ticket>`, never silently pass.
3. **Include a matching negative.** Every positive assertion is
   paired with an assertion that fails when the feature is broken.
4. **Emit copy-pasteable evidence** — body, screenshot, frame, DB
   row, log excerpt. Boolean pass/fail is insufficient.
5. **Verify "fails when feature is removed."** Author runs locally
   with the feature commented out; the test MUST FAIL. If it still
   passes, it's a bluff — delete and rewrite.
6. **No blind shells.** No `&& echo PASS`, `|| true`, `tee` exit
   laundering, `if [ -f file ]` without content assertion.

**Challenges in this submodule** must replay the user journey
end-to-end through the umbrella project's deliverables — never via
raw `curl` or third-party scripts. Sub-1-second Challenges almost
always indicate a bluff.

**HelixQA banks** declare executable actions
(`adb_shell:`, `playwright:`, `http:`, `assertVisible:`,
`assertNotVisible:`), never prose. Stagnation guard from Article I
§1.3 applies — frame N+1 identical to frame N for >10 s = FAIL.

**PR requirement:** every PR adding/modifying a test or Challenge in
this submodule MUST include a fenced `## Anti-Bluff Verification`
block with: (a) command run, (b) pasted output, (c) proof the test
fails when the feature is broken (second run with feature
commented-out showing FAIL).

**Cross-reference:** umbrella `CONSTITUTION.md` Article XI
(§§ 11.1 — 11.8).

<!-- END anti-bluff-testing addendum (Article XI) -->
