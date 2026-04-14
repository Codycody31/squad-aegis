# Security Policy

## Supported Versions

Squad Aegis is under active development and may contain breaking changes between releases.
Security fixes are generally provided for the most recent stable release and the `master` branch.

| Version / Branch | Supported |
| --- | --- |
| Latest release | :white_check_mark: |
| `master` | :white_check_mark: |
| Older releases | :x: |
| Unreleased forks / modified deployments | Best effort only |

Because this project is still evolving rapidly, older versions may not receive backported fixes.
Users should upgrade to the latest release as soon as possible.

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security problems.

Use one of the following private channels instead:

1. **Preferred:** GitHub's private vulnerability reporting/repository security advisories
2. **Fallback:** Contact the maintainer privately at **security@codycody31.dev**

## What to Include

Please include as much of the following as possible:

- A clear description of the issue
- Affected version(s), commit(s), branch(es), or deployment mode(s)
- Exact reproduction steps
- Proof-of-concept or exploit details
- Impact assessment
- Any relevant logs, screenshots, stack traces, or request/response samples
- Suggested remediation, if you have one

Also include relevant environment details where applicable:

- OS / distro
- Container vs. bare-metal deployment
- Reverse proxy in front of Squad Aegis
- Database/backend configuration
- Whether plugins, workflows, or custom integrations are enabled

## Scope

Security reports are especially relevant for issues involving:

- Authentication or authorization bypass
- Privilege escalation
- Multi-tenant or cross-server access violations
- Remote code execution
- Arbitrary file read/write
- SSRF
- Command injection
- SQL injection
- Stored/reflected / DOM XSS
- CSRF with meaningful impact
- Secret exposure
- Plugin isolation/plugin execution boundary failures
- Unsafe defaults in production deployment
- Supply chain compromise in build, release, or plugin loading paths

## Response Targets

I will make a reasonable effort to follow this timeline:

- **Initial acknowledgment:** within 7 days
- **Triage / severity assessment:** within 14 days
- **Status update:** at least every 14 days while the report is active
- **Fix or mitigation target:** as soon as reasonably possible, based on severity and complexity

These are targets, not guarantees.

## Disclosure Policy

Please follow coordinated disclosure.

- Do not publish details, proof-of-concept code, or exploit instructions before a fix or mitigation is available.
- Once the issue is resolved, I may publish a security advisory, release notes, and remediation guidance.
- Credit can be given to the reporter unless anonymity is requested.

## Safe Harbor

Good-faith security research is welcome.

When testing, please:

- Avoid privacy violations, data destruction, or service disruption
- Do not access, modify, or retain other users' data except as strictly necessary to demonstrate impact
- Use the minimum level of interaction required to prove the issue
- Stop testing and report immediately once you confirm a vulnerability

I will not pursue action against researchers who act in good faith and follow this policy.

## Out of Scope

The following are generally out of scope unless they demonstrate meaningful, real-world security impact:

- Missing best-practice headers without exploitability
- TLS / reverse-proxy misconfiguration outside this repository
- Version-only reports without a demonstrated exploit path
- Theoretical issues with no practical impact
- Social engineering, phishing, or physical attacks
- Denial of service requiring unrealistic resources
- Vulnerabilities only present in third-party services or infrastructure not maintained by this project
- Findings limited to local-only development setups with no security boundary crossed

## Deployment Notes

Squad Aegis is typically self-hosted. Final security posture depends in part on operator configuration, including:

- network exposure
- reverse proxy configuration
- secret management
- database access controls
- plugin usage
- update hygiene

Reports are still welcome for insecure defaults, footguns, or trust-boundary mistakes in the application and its official deployment artifacts.

## Preferred Remediation Format

When possible, remediation should include:

- the root cause
- affected components
- fixed version/commit
- upgrade instructions
- any temporary mitigations for users who cannot upgrade immediately
