# Security policy

## Reporting a vulnerability

Do not open a public issue for suspected authentication bypasses, leaked credentials, remote command execution, database exposure, or signing-key compromise. Send a private security advisory through GitHub Security Advisories for this repository and include:

- affected commit and component
- reproduction steps
- expected and observed behavior
- impact assessment
- suggested mitigation, when available

Do not include live access tokens, session cookies, private keys, user data, or production database contents. Revoke any credential that was exposed while reproducing the problem.

## Supported branch

Security fixes target the current `main` branch. Deployments should pin reviewed commits and run the CI workflow before rollout.

## Operational requirements

- Keep `JWT_SECRET`, Twitch secrets, HMAC keys, and database credentials outside Git.
- Use HTTPS in production.
- Register the exact Twitch callback URL including `/api`.
- Keep PostgreSQL on the internal Docker network without a host port mapping.
- Rotate secrets after suspected disclosure and restart every process that consumes them.
- Review production logs for accidental OAuth codes or credentials before sharing them.
