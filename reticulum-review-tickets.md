# Reticulum support - review tickets

Draft for review, not filed yet.

- Source diff: `git diff main HEAD` (57 files, +5231/-28)
- Branch: `WIP_reticulum_v2.1` at `c7abecdad0`
- Note: `git diff main...HEAD` shows 356 files because commit `081d3e0d79` re-applies upstream commits without merge ancestry; see ticket 18.
- Priorities: P0 = merge blocker, P1 = high, P2 = medium, P3 = low.

---

## 1. [P0][licensing] Resolve Reticulum-Go license terms and attribution

**Where:** `modules/rngit/handlers.go:1-2`, `modules/rngit/pages_md.go:1-2`, `modules/rngit/pages_render.go:1-2`, `go.mod:19`

**Problem:** These files are substantial derivatives of Reticulum-Go (the near-verbatim handler logic and adapted markdown/micron renderers), but they carry `Copyright 2026 The Gitea Authors` and `SPDX-License-Identifier: MIT`. Upstream uses `LicenseRef-Reticulum` and a custom "Reticulum License" with notice-retention and field-of-use conditions, including restrictions on AI/ML training use.

**Impact:** The current headers remove the upstream notice and relicense copied code as MIT. That is incompatible with the upstream notice-retention condition and the custom terms propagate to Gitea distributions. Merge blocker pending legal review.

**Proposed fix:** Get legal review. Preserve the upstream copyright and `LicenseRef-Reticulum` notices on all derived files and comply with the notice-retention condition, or replace the copied code with a clean-room implementation or a thin dependency wrapper. Do not relicense copied portions as MIT.

**Acceptance criteria:**
- [ ] Legal sign-off recorded.
- [ ] Derived files retain the upstream notice and correct SPDX identifier.
- [ ] No MIT claim over Reticulum-Go-derived code.
- [ ] Distribution implications of the Reticulum-Go dependency documented.

**Tests:** Not applicable; add a license header check if the project has one.

---

## 2. [P0][security] Enforce repository write policy and hooks for RNS pushes and deletes

**Where:** `modules/rngit/access.go:51-67`, `modules/rngit/handlers.go:128-235`, `modules/rngit/node.go:162-175`

**Problem:** The RNS write path checks only `GetIndividualUserRepoPermission` and then updates refs directly with `git fetch`/`git update-ref`. It does not enforce protected branches or tags, force-push rules, required approvals, signed-commit rules, archived repositories, mirrors, Gitea pre-receive hooks, or LFS handling.

**Impact:** A user with write access can force-push or delete a protected branch or tag, mutate an archived or mirror repository, and bypass every policy that normal Git-over-HTTP/SSH applies. The post-receive replacement only syncs branches and sends notifications.

**Proposed fix:** Route writes through the existing repository/hook pipeline, or add explicit checks equivalent to the normal receive path:
- reject writes to `IsArchived` and `IsMirror` repositories,
- enforce protected-branch/tag rules and force-push policy,
- run required pre-receive hooks,
- handle or explicitly reject LFS pushes,
- keep the ref update and post-receive processing consistent.

**Acceptance criteria:**
- [ ] Pushes and deletes to archived/mirror repositories are rejected.
- [ ] Protected branches and tags cannot be force-pushed or deleted through RNS.
- [ ] Pre-receive policy hooks run for RNS writes.
- [ ] LFS behavior is defined and enforced.
- [ ] Tests cover each rejection path and a successful allowed push.

**Tests:** Extend `modules/rngit/handlers_access_test.go` and `access_test.go` with archived/mirror/protected cases and success paths.

---

## 3. [P1][security] Reject deactivated and login-prohibited users on RNS paths

**Where:** `modules/rngit/access.go:35-42`, `modules/rngit/pages_access.go:29-36`; compare `routers/private/serv.go:58,218`

**Problem:** The identity hash is mapped to a user with `GetUserByID`, which does not check `IsActive` or `ProhibitLogin`. Normal Git authentication rejects both flags. `GetIndividualUserRepoPermission` also has no active/login check.

**Impact:** Deactivated or login-prohibited accounts keep RNS Git and page access until the account is deleted, bypassing an administrative control.

**Proposed fix:** Use `GetValidDoerUserByID` or add explicit `IsActive`/`ProhibitLogin` checks in both the Git and page identity resolution paths. Return the same non-revealing errors used for unregistered identities.

**Acceptance criteria:**
- [ ] Inactive and prohibited users get the same response as unregistered identities.
- [ ] Active users are unaffected.
- [ ] Tests cover inactive and prohibited users on list/fetch/push/delete and page access.

---

## 4. [P1][security] Add proof of possession when registering an RNS identity hash

**Where:** `models/asymkey/rns_identity_add.go:15-70`, `routers/api/v1/user/rns_identity.go:103-123`, `routers/web/user/setting/keys.go:256-290`

**Problem:** Registration accepts any syntactically valid 32-hex identity hash with no proof that the registrant controls the corresponding RNS identity.

**Impact:** Anyone can squat a public identity hash or bind someone else's hash to their account. The remote hash is cryptographically authentic at connection time, so this is not a direct authentication bypass, but it enables squatting, misattribution, and denial of service for the legitimate owner.

**Proposed fix:** Require a signed challenge using the RNS identity private key, an ownership proof generated by the client, or an administrator approval flow. Store the verification state if approval is asynchronous.

**Acceptance criteria:**
- [ ] A hash cannot be registered without proving control of the identity.
- [ ] Replay of a proof is rejected.
- [ ] Existing registrations have a migration or re-verification path.
- [ ] Tests cover valid proof, invalid proof, replay, and duplicate hash.

---

## 5. [P1][security] Bound memory use when fetching bundles and serving release artifacts

**Where:** `modules/rngit/handlers.go:69-86`, `modules/rngit/pages_files.go:150-199`

**Problem:** Fetch reads the entire generated bundle with `os.ReadFile`. Release artifact serving reads the entire attachment with `io.ReadAll` and has no size cap.

**Impact:** A large repository fetch or a large release attachment can allocate unbounded memory and exhaust the server. Release attachments may be attacker-controlled.

**Proposed fix:** Add configurable size limits and stream responses where the protocol allows it. For bundles, prefer returning a file-backed reader instead of `os.ReadFile`. For artifacts, use `http.ServeContent`/`io.Copy` with a limit and return a clear error when the limit is exceeded.

**Acceptance criteria:**
- [ ] Oversized fetches and artifacts fail with a clear response instead of allocating unbounded memory.
- [ ] Limits are configurable and documented.
- [ ] Tests cover the limit boundary and error path.

---

## 6. [P2][bug] Make post-receive processing robust to snapshot failures and partial pushes

**Where:** `modules/rngit/postreceive.go:20-45`, `modules/rngit/handlers.go:174-214`

**Problem:** `snapshotRefs` returns `nil` on error and `postReceive` uses the result without a nil guard. If the `after` snapshot fails, every old ref appears deleted and can be marked deleted in the DB. The multi-ref `update_ref` loop is also non-transactional: refs before a failure are already changed, but `postReceive` is skipped.

**Impact:** A transient Git error can corrupt branch DB state or produce false deletion events. A partial multi-ref push leaves refs and the DB inconsistent.

**Proposed fix:** Make `snapshotRefs` return an error and abort post-processing when either snapshot fails. For multi-ref operations, validate all operations first and apply them atomically where possible, or call post-receive with an accurate before/after even on partial application.

**Acceptance criteria:**
- [ ] A failed before/after snapshot never produces false add/delete events.
- [ ] Partial multi-ref failures leave refs and DB consistent, or are rolled back.
- [ ] Tests inject snapshot and mid-loop failures.

---

## 7. [P2][bug] Adopt the first pushed branch when the repository has no default branch

**Where:** `modules/rngit/postreceive.go:129-144`

**Problem:** `ensureDefaultBranch` keeps an existing default or guesses `main`/`master`. It does not adopt the first pushed branch, unlike Gitea's normal post-receive hook, and it runs on every branch update.

**Impact:** Pushing a first branch with another name leaves the repository without a correct default branch and can break clone/display expectations.

**Proposed fix:** When the repository has no default branch and the push adds branches, adopt the first pushed branch. Only reconcile HEAD when needed.

**Acceptance criteria:**
- [ ] First push of a non-main/master branch becomes the default.
- [ ] Existing defaults are not overwritten.
- [ ] Tests cover first push, existing default, and branch update.

---

## 8. [P2][reliability] Do not abort Gitea startup and harden rngit lifecycle

**Where:** `routers/init.go:63-77,154`, `modules/rngit/node.go:74-136`

**Problem:** `mustInitCtx(ctx, rngit.Run)` calls `log.Fatal` on any error. A malformed Reticulum config or node start failure takes down all of Gitea. Partial start failures leave `s.node`/`s.dest` set, the page destination can leak, `Stop` does not wait for the announce goroutine or close destinations, and the package-global `defaultServer` is accessed without synchronization.

**Impact:** An optional feature can prevent the whole instance from starting, and shutdown/restart can race with the announce loop.

**Proposed fix:** Log rngit startup failures and disable the feature instead of exiting. Clean up all partially created resources on start failure, wait for goroutines on stop, close destinations explicitly if the library does not, and guard `defaultServer` with a mutex or atomic.

**Acceptance criteria:**
- [ ] Invalid rngit configuration does not prevent Gitea from starting.
- [ ] Failed starts leave no live node/destination/goroutine.
- [ ] Stop is race-free; run with `-race`.
- [ ] Shutdown/restart tests cover partial-failure and success paths.

---

## 9. [P2][bug] Align `ANNOUNCE_INTERVAL` behavior with its documentation

**Where:** `modules/rngit/node.go:102,115`, `modules/setting/rngit.go`, `custom/conf/app.example.ini:815-840`

**Problem:** `Start` always calls `s.dest.Announce` before checking the interval. Only the repeating loop is gated by `AnnounceInterval > 0`. A negative value therefore still announces once, although the configuration comments say negative disables announcing. Zero and negative currently behave the same.

**Impact:** Operators cannot disable announcements as documented.

**Proposed fix:** Define the intended semantics and implement them consistently: either skip the initial announce when the interval is negative, or update the comments to say negative/zero announce once. Add a test for zero, positive, and negative values.

**Acceptance criteria:**
- [ ] Code and configuration docs agree.
- [ ] Negative/zero/positive behavior is covered by tests.

---

## 10. [P2][api] Resolve DELETE 204-vs-404 contract mismatch for RNS identities

**Where:** `routers/api/v1/user/rns_identity.go:148-172`, `models/asymkey/rns_identity.go:67-85`, `tests/integration/api_rns_identities_test.go`

**Problem:** The handler returns `204 No Content` even when the id does not exist or belongs to another user, because the model treats a missing row as a no-op. Swagger declares `404`, and the integration test codifies the `204` behavior.

**Impact:** API clients cannot distinguish success from a missing/foreign id, and the generated contract is wrong.

**Proposed fix:** Choose one contract. If deleting a missing/foreign id should be idempotent, document `204` in swagger and explain it. If it should be `404`, propagate `ErrKeyNotExist` from the model and update the test.

**Acceptance criteria:**
- [ ] Handler, model, swagger, and tests agree.
- [ ] Foreign ids cannot delete another user's identity (already enforced by owner scoping).
- [ ] Test covers missing, foreign, and successful delete.

---

## 11. [P2][performance] Paginate branch/tag listing on RNS pages

**Where:** `modules/rngit/pages_handlers.go:474-497`

**Problem:** The refs page loads all branches with `GetBranchNames(0, 0)` and calls `GetTagInfos` twice, the second time with an unbounded total. Tags and branches are not paginated.

**Impact:** Repositories with many refs produce slow responses and large memory use, and the NomadNet page can become unusable.

**Proposed fix:** Add bounded pagination for branches and tags, fetch tag metadata once, and reuse the result. Reuse the existing commits pagination pattern.

**Acceptance criteria:**
- [ ] Branch and tag listings are paginated.
- [ ] `GetTagInfos` is called once per page.
- [ ] Tests cover multiple pages and an empty repository.

---

## 12. [P2][bug] Validate RNS identity names in the model and fix error wrapping/races

**Where:** `models/asymkey/rns_identity_add.go:40-70`, `models/asymkey/rns_identity.go:67-85`

**Problem:** The model trims names but does not enforce non-empty or <=50 characters. The check-then-insert uniqueness logic can surface a raw DB unique error on a race instead of the typed already-used error. Error wrappers name the callee (`CheckRNSIdentityHash:`, `GetRNSIdentityForUserByID:`, `db.WithTx:`) rather than the enclosing function, contrary to the repository convention.

**Impact:** Invalid rows can be created outside the HTML/API validators, and callers can receive untyped 500 errors. Error messages are misleading.

**Proposed fix:** Enforce name validation in `AddRNSIdentity`, map DB duplicate errors to `ErrRNSIdentityAlreadyUsed`/`ErrKeyNameAlreadyUsed`, and wrap errors with the enclosing function name.

**Acceptance criteria:**
- [ ] Empty and overlong names are rejected at the model layer.
- [ ] Concurrent duplicate inserts return typed errors, not raw DB errors.
- [ ] Error prefixes name the enclosing function.
- [ ] Unit tests cover validation and concurrent duplicate inserts.

---

## 13. [P2][security/rendering] Harden NomadNet micron rendering for repo-controlled content

**Where:** `modules/rngit/pages_handlers.go:240-252,331-332`, `modules/rngit/pages_md.go:188-245,247-249,301-303,338-346,506-559`

**Problem:** Symlink targets are interpolated into micron output without escaping, and `.mu` files are rendered raw by design. Markdown link labels are not escaped for `[`/`]`/backslash, and the NUL placeholder scheme can collide with content. Width/truncation/wrapping use byte lengths, which can split multibyte UTF-8.

**Impact:** Repository content can inject micron markup or produce broken links, and non-ASCII content can be corrupted in tables and wrapped text.

**Proposed fix:** Escape all repo-controlled values (including symlink targets) for micron, use a collision-resistant placeholder scheme, and measure/truncate by runes or display width. Document the intentional raw-`.mu` behavior and whether it is trusted.

**Acceptance criteria:**
- [ ] Symlink targets cannot inject micron syntax.
- [ ] Link labels and NUL-containing content render safely.
- [ ] Multibyte UTF-8 is preserved through table truncation and wrapping.
- [ ] Tests cover injection attempts and non-ASCII content.

---

## 14. [P2][dependencies] Review the Reticulum-Go dependency footprint and Go directive change

**Where:** `go.mod:3,19,112-114`

**Problem:** The feature adds Reticulum-Go v1.4.1 plus indirect Quad4 bzip2/msgpack/tagparser, quic-go/webtransport, godbus, landlock, serial, and vsock dependencies. `go.mod` also changes `go 1.27` to `go 1.27.1`, while the toolchain is go1.27.2.

**Impact:** Large supply-chain surface, license review burden, and a patch-level `go` directive that may violate repository policy and force unnecessary toolchain upgrades.

**Proposed fix:** Confirm every new module is necessary, record its license, run the vulnerability/license checks, and revert the `go` directive to `1.27` unless a patch-level minimum is required. Prefer upstream dependencies over forked forks where possible.

**Acceptance criteria:**
- [ ] License inventory for all new direct/indirect modules.
- [ ] No unnecessary forks or unused dependencies.
- [ ] `go` directive follows repository policy.
- [ ] `go mod tidy` and dependency checks pass.

---

## 15. [P2][testing] Add success-path and policy tests for RNS Git operations

**Where:** `modules/rngit/access_test.go`, `modules/rngit/handlers_access_test.go`, `modules/rngit/handlers_test.go`

**Problem:** Current tests cover guard paths and feature toggles but not successful fetch/push/delete/update-ref flows, and there are no tests for protected branches, archived/mirror repositories, inactive users, or size limits.

**Impact:** Core write behavior and the security fixes above are unverified.

**Proposed fix:** Add focused integration-style tests for successful list/fetch/push/delete, multi-ref updates, protected-branch rejection, archived/mirror rejection, inactive/prohibited users, snapshot failures, and size limits.

**Acceptance criteria:**
- [ ] Successful operations are covered end to end.
- [ ] Each policy rejection has a test.
- [ ] Tests run in the normal `make test` suite and stay within the repository's time budget.

---

## 16. [P3][docs] Document the Reticulum feature, configuration, and identity setup

**Where:** `docs/` (no changes), `custom/conf/app.example.ini:815-840`

**Problem:** No documentation was added for enabling rngit, Reticulum configuration, identity paths, announcement behavior, NomadNet pages, identity registration, or security limitations.

**Impact:** Operators cannot configure the feature safely, and the documented semantics (for example negative `ANNOUNCE_INTERVAL`) can drift from code.

**Proposed fix:** Add a configuration/usage page covering prerequisites, example config, identity hash registration, clone URL behavior, page exposure, and known limitations.

**Acceptance criteria:**
- [ ] Docs cover enabling/disabling and every new setting.
- [ ] Identity registration and verification steps are documented.
- [ ] Security limitations and the protection model are explicit.

---

## 17. [P3][cleanup] Remove stale comments, typos, unused code, and copy-paste text

**Where:** `modules/rngit/handlers.go:165,168`, `modules/rngit/pages_handlers.go:26-29,606-618`, `web_src/js/features/repo-common.ts:56`, `options/locale/locale_en-US.json`

**Problem:** Several small issues remain:
- `FetchBundle` failures report `Could not verify bundle`.
- `treeEntriesPerPage` is unused.
- `entriesSort`'s comment claims directories last, but the implementation is a plain string sort.
- The clone-section comment still says only HTTPS/SSH are supported.
- The Keys page title still says "SSH/GPG keys" although it includes RNS.
- The invalid-hash locale text says "lowercase" although uppercase input is normalized.

**Impact:** Misleading diagnostics and maintenance friction.

**Proposed fix:** Fix the messages, delete unused code, align comments with behavior, and update the user-facing labels.

**Acceptance criteria:**
- [ ] No stale or contradictory comments remain in the changed files.
- [ ] Unused constants are removed.
- [ ] Error messages and labels are accurate.
- [ ] `make lint-go`, `make lint-js`, and locale checks pass.

---

## 18. [P3][process] Rebase the feature branch and clean up commit history

**Where:** commit `081d3e0d79`, branch `WIP_reticulum_v2.1`

**Problem:** `081d3e0d79` re-applies about 17 upstream commits without merge ancestry, so `git diff main...HEAD` shows 356 files instead of the real 57-file feature diff. Commit messages such as "Feat: It works" and "Feat: pages?" do not follow the repository's Conventional Commits convention.

**Impact:** The PR diff is misleading, review is harder, and upstream merge/rebase conflicts are likely.

**Proposed fix:** Rebase onto current `main` so the branch contains only the Reticulum changes, and use Conventional Commits with the `enhance` type for user-facing changes. Do not rewrite shared history without coordination.

**Acceptance criteria:**
- [ ] `git diff main...HEAD` matches the intended feature diff.
- [ ] Commit messages follow the repository convention.
- [ ] No unrelated upstream changes remain in the feature commits.
