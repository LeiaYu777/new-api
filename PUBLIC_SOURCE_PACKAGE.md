# Public Source Package

This repository is intended to be the public Corresponding Source package for this modified version of New API when published at the configured `SOURCE_CODE_URL`.

## Included

The public source package includes:

- application source code
- frontend source code and build configuration
- backend source code and build configuration
- build scripts
- deployment scripts
- Dockerfile and Docker-related example configuration
- non-sensitive environment examples such as `.env.example` and `.env.lean.example`
- NOTICE and modification records
- AGPLv3 compliance notes
- documentation required to understand, build, deploy, and operate the modified program

## Not Included

The public source package does not include:

- production `.env` files
- `.env.production` or other private runtime configuration
- API keys
- model provider keys
- customer accounts
- customer data
- billing records or billing exports
- private user data
- database passwords
- database DSNs
- SQLite production databases
- database dump files
- logs containing private data
- server SSH keys
- TLS private keys
- Docker registry passwords
- cloud credentials
- any other secrets or customer-specific confidential information

## Runtime Configuration

Set the following environment variables in the private deployment environment, not in committed source files:

```env
SOURCE_CODE_URL=https://github.com/LeiaYu777/new-api
LEGAL_CONTACT_EMAIL=lei.yu@qq.com
MODIFIER_NAME=leia
MODIFIED_VERSION=lean-2026.05
MODIFIED_DATE=2026-05-06
MODIFICATION_SUMMARY=2C4G lean deployment, startup task configuration, ByteDance/Volcengine model usage configuration, billing/account workflow customization.
```

If `SOURCE_CODE_URL` is not configured, the application should continue to run and instruct users to contact the administrator for the corresponding source code.
