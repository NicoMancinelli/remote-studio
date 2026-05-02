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
2. Run `make test` locally — it runs `shellcheck` plus the `bats` suite. Both must pass.
3. If you change `res.sh`, run it interactively with `bash res.sh` and verify the TUI still loads
4. Update `CHANGELOG.md` under the unreleased section
5. Open the PR with a clear description and link to any related issue

## Code style

- **Bash**: 4-space indentation, follow existing patterns. New subcommands must support a non-interactive CLI mode (the applet calls them headlessly).
- **GJS (applet)**: keep it async — never block the panel thread on shell commands.

## Code of conduct

Be respectful. Disagreements are fine; personal attacks are not.
