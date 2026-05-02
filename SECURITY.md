# Security Policy

## Supported versions

Only the latest tagged release receives security updates.

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Email Nico Mancinelli at `nicomancinelli@gmail.com` with:

- A description of the vulnerability
- Steps to reproduce
- Suggested fix if you have one

You should expect a response within 5 business days. If the vulnerability is confirmed:

1. We will agree on a coordinated disclosure timeline (typically 30–90 days)
2. A fix will be developed in a private branch
3. A patch release will be published with a CVE if applicable
4. Credit will be given in the release notes unless you prefer to remain anonymous

## Scope

In scope:
- `res.sh` and `install.sh` (privilege escalation, command injection, path traversal)
- The Cinnamon applet (anything that runs with the user's session privileges)
- The `.deb` package (unsafe postinst, world-writable files)
- The `install-remote-studio.sh` curl-pipe-bash installer

Out of scope:
- Vulnerabilities in upstream dependencies (RustDesk, Tailscale, Cinnamon, X11)
- Misconfiguration by the user (e.g. weak RustDesk password)
- Issues that require physical access or root on the target machine
