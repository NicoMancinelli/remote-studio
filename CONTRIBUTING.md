# Contributing to Remote Studio

Thanks for your interest in contributing. Remote Studio is a small project — issues, feature requests, and PRs are all welcome.

## Reporting bugs

Open a [GitHub issue](https://github.com/NicoMancinelli/remote-studio/issues/new) and include:

- Output of `res doctor`
- Output of `res version`
- Linux Mint / Cinnamon version (`cat /etc/os-release && cinnamon --version`)
- The exact command or TUI action that triggered the bug
- Relevant lines from `~/.remote_studio.log`

## Suggesting features

Open an issue with the **enhancement** label. Describe the use case first, then the proposed behaviour.

## Submitting a PR

1. Fork the repo and create a feature branch (`git checkout -b feat/my-thing`)
2. Run `make ci` locally — it runs ShellCheck, bats, shell syntax checks, applet syntax checks, status JSON, and installer dry-run. All must pass.
3. If you change installer, package, or release behavior, run `make release-check`
4. If you change `res.sh` or `lib/*.sh`, run it interactively with `bash res.sh` and verify the TUI still loads
5. If you change status output, preserve both contracts: pipe-delimited `res status` for the applet and JSON `res status --json` for automation
6. Update `CHANGELOG.md` under the unreleased section
7. Open the PR with a clear description and link to any related issue

## Code style

- **Bash**: 4-space indentation, follow existing patterns. New subcommands must support a non-interactive CLI mode (the applet calls them headlessly).
- **GJS (applet)**: keep it async — never block the panel thread on shell commands. Keep labels compact and put detail in tooltips or grouped menu actions.
- **Docs**: keep the README as the product overview and `docs/quickstart.md` as the maintained operational guide. Do not duplicate a second full quick start in `docs/quick-start.md`.

## Code of conduct

Be respectful. Disagreements are fine; personal attacks are not.
