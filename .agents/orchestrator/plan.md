# Remote Studio Modernization Plan

## Overview
This plan defines the modernization of Remote Studio. We rewrite the Python/Bash backend into a statically-linked Go binary and build 6 core OS integrations.

## Strategy
We use the **Project Pattern** (Dual Track):
1. **E2E Testing Track**: Derives requirements from the original codebase and specifications. Builds a 4-tier opaque-box test suite. Publishes `TEST_READY.md`.
2. **Implementation Track**:
   - **Milestone 2 (Go Rewrite Foundation)**: Port the Python daemon and Bash CLI into a single Go binary (`res`).
   - **Milestones 3-8 (OS Integrations)**: Build Wayland support, systemd socket activation, PipeWire virtual audio sinks, `uinput` virtual KVM, VA-API/NVENC dynamic checks, and TOML configuration parsing.
   - **Milestone 9 (Integration Gate)**: Verify 100% test pass on all E2E tests (Tiers 1-4), then run Phase 2 adversarial coverage hardening (Tier 5).

## Verification Plan
Each subtask will be executed by subagents and verified by a reviewer and challenger before closing.
The final integration will be validated against the E2E test suite.
A Forensic Auditor will perform static analysis and runtime tracing to ensure code integrity (no hardcoded test hacks, no facade implementations).
