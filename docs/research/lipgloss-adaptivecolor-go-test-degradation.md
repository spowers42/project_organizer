# lipgloss `AdaptiveColor` behavior under `go test`

Date: 2026-09-11

**Question:** Does lipgloss's `AdaptiveColor` / automatic color-profile detection reliably degrade to plain, ANSI-free output when styles are exercised inside `go test` (outside a real TTY)? If not automatic in every case, what is the exact environment variable or API call to force it deterministically, without a string-stripping wrapper?

## Answer summary

- In this repo's pinned version (lipgloss v1.1.0), `AdaptiveColor` and all other lipgloss coloring/styling go through **`github.com/muesli/termenv`**, not `github.com/charmbracelet/colorprofile`. `colorprofile` is present in `go.mod` only as a transitive, indirect dependency pulled in via `bubbletea → lipgloss → charmbracelet/x/cellbuf`, and is unrelated to `AdaptiveColor`/`Style.Render` resolution.
- Degradation to a colorless ("Ascii") profile **is automatic when stdout is not a TTY** (e.g., piped/redirected, as under most CI runners and `go test ./... > log.txt`), because termenv's TTY check short-circuits to `Ascii` before consulting `TERM`/`COLORTERM`.
- **This automatic degradation is not guaranteed for every `go test` invocation.** If a developer runs `go test` directly in an interactive terminal without redirecting output, `os.Stdout` is still the real terminal file descriptor, `isatty.IsTerminal()` returns `true`, and termenv will do full TERM/COLORTERM-based detection — potentially producing real ANSI/color codes in test output/assertions.
- The reliable, environment-based fix requested (no wrapper) is to set **`NO_COLOR=1`** (or any non-empty value) in the test process's environment. termenv's `EnvColorProfile()` — which is exactly what lipgloss's default `Renderer.ColorProfile()` calls — checks `NO_COLOR` *before* doing any TTY/TERM detection, and forces `termenv.Ascii` unconditionally when it is set, regardless of whether stdout is a TTY.
- `CLICOLOR_FORCE=0` (mentioned as a candidate in the ticket) does **not** by itself force plain output in v0.16.0 termenv — it only cancels a `CLICOLOR=0`-driven no-color state, and has no effect on the default TTY/TERM detection path. It is not a substitute for `NO_COLOR`.
- The equivalent, more explicit **API-level** fix is `lipgloss.SetColorProfile(termenv.Ascii)` (package-level, defaulting to lipgloss's global renderer), or `renderer.SetColorProfile(termenv.Ascii)` on a specific `*lipgloss.Renderer`. This is documented in lipgloss's own source as existing "mostly for testing purposes" and is the most deterministic option because it bypasses environment/TTY detection entirely rather than depending on how the test runner or CI wires environment variables.

## Default behavior under `go test`

lipgloss v1.1.0's default renderer is a package-level singleton built directly on `termenv.DefaultOutput()` (which wraps `os.Stdout`):

```go
// github.com/charmbracelet/lipgloss@v1.1.0/renderer.go:10-14
var renderer = &Renderer{
	output: termenv.DefaultOutput(),
}
```

`Style.Render` resolves the color profile from that renderer on every call (no persistent global "test mode" flag):

```go
// github.com/charmbracelet/lipgloss@v1.1.0/style.go:244-247
p            = s.r.ColorProfile()
te           = p.String()
...
```

`Renderer.ColorProfile()` caches (via `sync.Once`) the result of `termenv`'s **environment-aware** detector, `Output.EnvColorProfile()`:

```go
// github.com/charmbracelet/lipgloss@v1.1.0/renderer.go:70-79
if !r.explicitColorProfile {
	r.getColorProfile.Do(func() {
		r.colorProfile = r.output.EnvColorProfile()
	})
}
```

`AdaptiveColor` resolves through the exact same path — it just picks `Light` or `Dark` first, then calls `Color(...).color(r)`, which calls `r.ColorProfile().Color(string(c))`:

```go
// github.com/charmbracelet/lipgloss@v1.1.0/color.go:99-104
func (ac AdaptiveColor) color(r *Renderer) termenv.Color {
	if r.HasDarkBackground() {
		return Color(ac.Dark).color(r)
	}
	return Color(ac.Light).color(r)
}

// github.com/charmbracelet/lipgloss@v1.1.0/color.go:47-49
func (c Color) color(r *Renderer) termenv.Color {
	return r.ColorProfile().Color(string(c))
}
```

`termenv.Output.EnvColorProfile()` first checks `NO_COLOR`/`CLICOLOR`, and only falls through to plain TTY/`TERM` detection (`ColorProfile()`) if none of those env vars apply:

```go
// github.com/muesli/termenv@v0.16.0/termenv.go:99-108
func (o *Output) EnvColorProfile() Profile {
	if o.EnvNoColor() {
		return Ascii
	}
	p := o.ColorProfile()
	if o.cliColorForced() && p == Ascii {
		return ANSI
	}
	return p
}
```

`ColorProfile()` (the plain, non-env-aware detector) is where the TTY check lives, and it is unconditional and first:

```go
// github.com/muesli/termenv@v0.16.0/termenv_unix.go:23-26
func (o *Output) ColorProfile() Profile {
	if !o.isTTY() {
		return Ascii
	}
	...
```

`isTTY()` also treats CI as non-interactive regardless of the fd:

```go
// github.com/muesli/termenv@v0.16.0/termenv.go:28-40
func (o *Output) isTTY() bool {
	if o.assumeTTY || o.unsafe {
		return true
	}
	if len(o.environ.Getenv("CI")) > 0 {
		return false
	}
	if f, ok := o.Writer().(*os.File); ok {
		return isatty.IsTerminal(f.Fd())
	}
	return false
}
```

Finally, when the resolved profile is `Ascii`, `termenv.Style.Styled` refuses to emit *any* escape sequence at all — not just for colors, but for bold/underline/etc. too — which is the actual mechanism that guarantees plain output once the profile is `Ascii`:

```go
// github.com/muesli/termenv@v0.16.0/style.go:43-49
func (t Style) Styled(s string) string {
	if t.profile == Ascii {
		return s
	}
	...
```

And `Profile.Convert`/`Profile.Color` independently also return `NoColor{}` for the `Ascii` profile:

```go
// github.com/muesli/termenv@v0.16.0/profile.go:46-49
func (p Profile) Convert(c Color) Color {
	if p == Ascii {
		return NoColor{}
	}
	...
```

**Net effect:** whether a `go test` run produces plain or colored output for `AdaptiveColor`/`Style.Render` depends entirely on whether `os.Stdout` for the test binary is a real TTY at the moment `ColorProfile()` is first evaluated (results are cached per-process via `sync.Once`, `renderer.getColorProfile`), *unless* `NO_COLOR`/`CLICOLOR` env vars are set, which short-circuit that check entirely. Piped/redirected/CI test runs (`go test ./... > file`, most CI systems, `CI=1` set) reliably get `Ascii` automatically. An interactive `go test` run in a real terminal, with a color-capable `TERM`/`COLORTERM` and no `NO_COLOR`, will **not** automatically degrade — it will produce real ANSI codes.

## Recommended mechanism to force it

Two options, both satisfying "environment/profile-detection, not a string-stripping wrapper":

### 1. Environment variable — `NO_COLOR=1` (simplest, works for CI and local)

Set it for the test process (Makefile, CI workflow env, or `os.Setenv("NO_COLOR", "1")` in a `TestMain`/`init()` **before** any lipgloss render call, since the profile is cached via `sync.Once` on first use):

```go
// github.com/muesli/termenv@v0.16.0/termenv.go:68-70
func (o *Output) EnvNoColor() bool {
	return o.environ.Getenv("NO_COLOR") != "" || (o.environ.Getenv("CLICOLOR") == "0" && !o.cliColorForced())
}
```

Any non-empty value works (it's a non-empty-string check, not a boolean parse, in this termenv version). This forces `termenv.Ascii` unconditionally, bypassing the TTY check entirely — reliable in every environment, interactive or not.

Caveat confirmed from source: `CLICOLOR_FORCE=0` does **not** achieve this. `cliColorForced()` only returns true when `CLICOLOR_FORCE` is set to something *other than* `"0"`, and it only matters in combination with `CLICOLOR=0`:

```go
// github.com/muesli/termenv@v0.16.0/termenv.go:110-115
func (o *Output) cliColorForced() bool {
	if forced := o.environ.Getenv("CLICOLOR_FORCE"); forced != "" {
		return forced != "0"
	}
	return false
}
```

`TERM=dumb` also works as a fallback path (when combined with unset/non-matching `COLORTERM`, in a real-or-assumed TTY it falls through every case in `termenv_unix.go`'s `ColorProfile()` switch to the final `return Ascii`), but it is more fragile than `NO_COLOR` because it depends on no other branch matching first — `NO_COLOR` is the purpose-built, unconditional knob and is the recommended one.

### 2. API call — `lipgloss.SetColorProfile(termenv.Ascii)` (most deterministic; bypasses detection entirely)

Exact v1.1.0 signature:

```go
// github.com/charmbracelet/lipgloss@v1.1.0/renderer.go:126-128
func SetColorProfile(p termenv.Profile) {
	renderer.SetColorProfile(p)
}
```

which sets an explicit flag that skips detection from then on:

```go
// github.com/charmbracelet/lipgloss@v1.1.0/renderer.go:102-108
func (r *Renderer) SetColorProfile(p termenv.Profile) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.colorProfile = p
	r.explicitColorProfile = true
}
```

The doc comment on this exact function in this exact version states it plainly: *"This function exists mostly for testing purposes so that you can assure you're testing against a specific profile."* Available profiles per source comment: `termenv.Ascii`, `termenv.ANSI`, `termenv.ANSI256`, `termenv.TrueColor`. Call `lipgloss.SetColorProfile(termenv.Ascii)` once (e.g., in `TestMain`, or an `init()` in a test-only file) — no env var, no TTY dependency, no wrapper, and it is thread-safe (guarded by `r.mtx`).

If tests exercise a custom `*lipgloss.Renderer` (e.g., created via `lipgloss.NewRenderer(w)` for a specific writer), call `.SetColorProfile(termenv.Ascii)` on that renderer instance instead of/in addition to the package-level default.

Recommendation: prefer the API call (`lipgloss.SetColorProfile(termenv.Ascii)`) for determinism independent of how the test runner sets up stdout/env; use `NO_COLOR=1` as a belt-and-braces CI-level setting as well, since it's cheap and standard.

## go.mod / go.sum findings

- `github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect` — confirmed **indirect** in `go.mod` (also confirmed via `go mod why github.com/charmbracelet/colorprofile`, which traces the chain: `github.com/spowers42/project_organizer/tui → github.com/charmbracelet/bubbletea → github.com/charmbracelet/lipgloss → github.com/charmbracelet/x/cellbuf → github.com/charmbracelet/colorprofile`).
- `github.com/charmbracelet/lipgloss v1.1.0 // indirect` — also currently marked indirect in this repo's `go.mod` (no `.go` file in the repo imports `lipgloss` or `colorprofile` directly yet — confirmed by `grep -rln "lipgloss"`/`"AdaptiveColor"` across the repo returning no hits at research time). This means the AdaptiveColor/testing question is prospective — nothing in the repo currently calls lipgloss.
- `github.com/muesli/termenv v0.16.0 // indirect` — the package that actually implements everything relevant to `AdaptiveColor` degradation in lipgloss v1.1.0. It is currently not a direct repo dependency either.
- **`colorprofile` should not become direct on account of the recommended mechanism.** The recommended API call (`lipgloss.SetColorProfile`) and env var (`NO_COLOR`) both operate through `termenv`, which lipgloss v1.1.0 imports and re-exports profile constants from (`termenv.Ascii`, etc.) — so a test file using `lipgloss.SetColorProfile(termenv.Ascii)` will need a **direct** dependency on `github.com/muesli/termenv` (for the `termenv.Ascii` constant) in addition to `github.com/charmbracelet/lipgloss`, once either package is actually imported by test code. `colorprofile` remains irrelevant to this mechanism in lipgloss v1.1.0 and does not need to become direct.
- Note for future lipgloss upgrades: charmbracelet has been migrating its ecosystem (bubbletea v2, lipgloss v2 betas) from `termenv` onto `colorprofile` directly. If this repo later upgrades lipgloss past v1.1.0 to a version built on `colorprofile` instead of `termenv`, this document's exact API/env findings would need to be re-verified against that version's source, per the same method used here (the `colorprofile.Detect`/`colorprofile.Env` functions in `github.com/charmbracelet/colorprofile@v0.2.3-.../env.go` already exist and follow the same `NO_COLOR`/`CLICOLOR`/`TERM=dumb`→`NoTTY` rules, so the recommended `NO_COLOR` env var would very likely still apply, but the exact API call would change, e.g. to a `colorprofile.Writer` wrapping stdout with an explicit `Profile` field).

## Source citations

All paths below are under `$(go env GOMODCACHE)` = `/var/home/hobbit/go/pkg/mod`.

- `github.com/charmbracelet/lipgloss@v1.1.0/renderer.go`
  - lines 10-14: package-level default `renderer` built on `termenv.DefaultOutput()`
  - lines 65-79: `Renderer.ColorProfile()` / package `ColorProfile()`, caches `r.output.EnvColorProfile()` via `sync.Once`
  - lines 86-108: `Renderer.SetColorProfile` / package `SetColorProfile(p termenv.Profile)`, doc comment "exists mostly for testing purposes"
  - lines 130-181: `HasDarkBackground`/`SetHasDarkBackground` (same explicit-override pattern)
- `github.com/charmbracelet/lipgloss@v1.1.0/color.go`
  - lines 45-49: `Color.color(r)` → `r.ColorProfile().Color(string(c))`
  - lines 87-104: `AdaptiveColor` type and `.color(r)` resolution (`HasDarkBackground` branch, then delegates to `Color.color`)
  - lines 116-136: `CompleteColor.color` (explicit ANSI/ANSI256/TrueColor per profile, `NoColor{}` default — degradation not automatic for this type, by design)
- `github.com/charmbracelet/lipgloss@v1.1.0/style.go`
  - lines 244-247: `Style.Render` resolves `p = s.r.ColorProfile()` fresh on every render call
  - lines 321-339: `fg.color(s.r)` / `bg.color(s.r)` feed into `termenv.Style.Foreground/Background`
- `github.com/muesli/termenv@v0.16.0/termenv.go`
  - lines 28-40: `Output.isTTY()` — `CI` env short-circuit, `isatty.IsTerminal(f.Fd())` check
  - lines 63-79: `EnvNoColor()` — `NO_COLOR`/`CLICOLOR` semantics
  - lines 81-108: `EnvColorProfile()` — `NO_COLOR`/`CLICOLOR` short-circuit before `ColorProfile()`/TTY detection
  - lines 110-115: `cliColorForced()` — why `CLICOLOR_FORCE=0` alone is a no-op
- `github.com/muesli/termenv@v0.16.0/termenv_unix.go`
  - lines 21-76: `Output.ColorProfile()` — `!isTTY() → Ascii` first check, then `TERM`/`COLORTERM` heuristics
- `github.com/muesli/termenv@v0.16.0/output.go`
  - lines 9-10, 54-87: `output = NewOutput(os.Stdout)`, `DefaultOutput()`, `NewOutput` (profile eagerly computed via `EnvColorProfile()` unless `WithProfile` option given)
  - lines 174-179: `HasDarkBackground()` default derivation from `BackgroundColor()`
- `github.com/muesli/termenv@v0.16.0/style.go`
  - lines 43-57: `Style.Styled` — `if t.profile == Ascii { return s }` unconditional escape suppression
  - lines 60-73: `Style.Foreground`/`Background`
- `github.com/muesli/termenv@v0.16.0/profile.go`
  - lines 11-22: `Profile` enum (`TrueColor`, `ANSI256`, `ANSI`, `Ascii`)
  - lines 45-77: `Profile.Convert` — `Ascii → NoColor{}` unconditional
  - lines 84-105: `Profile.Color(s string)` — delegates to `Convert`
- `github.com/charmbracelet/colorprofile@v0.2.3-0.20250311203215-f60798e515dc/env.go`
  - lines 16-30: `Detect(output io.Writer, env []string) Profile` doc/behavior (NO_COLOR/CLICOLOR/CLICOLOR_FORCE, `TERM=dumb` → `NoTTY`)
  - lines 67-135: `Env`, `colorProfile`, `envNoColor`, `cliColor`, `cliColorForced`, `envColorProfile` — parallel but distinct implementation from termenv's (uses `strconv.ParseBool` rather than non-empty-string check for `NO_COLOR`)
  - confirmed **not** on the call path for `lipgloss.AdaptiveColor`/`Style.Render` in v1.1.0 — only reached via `charmbracelet/x/cellbuf`'s `ConvertLink`/`ConvertStyle` (`link.go:4-9`, `style.go:4-16`), used by `bubbletea`'s screen rendering, not by lipgloss's own color resolution.
- Repo files consulted: `/var/home/hobbit/Development/project_organizer/go.mod`, `/var/home/hobbit/Development/project_organizer/go.sum`
- Commands run for verification: `go env GOMODCACHE`, `go mod why github.com/charmbracelet/colorprofile`, `grep -rln "lipgloss\|AdaptiveColor"` across the repo (no hits — no current direct usage in this codebase).
