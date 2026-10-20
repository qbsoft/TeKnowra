# Langflow editor address rollout

User-authorized scope: production “打开编排” opens https://flow.yihe.work:9222; local development remains http://127.0.0.1:7861. Preserve credentials, flow configurations, published releases and existing business services. No delivery packages.

- [x] Reuse existing frontend runtime configuration; add LANGFLOW_EDITOR_URL, with local default when unset.
- [x] Test explicit production origin and existing button call, local fallback, unsafe URL rejection and shell-to-JavaScript interpolation, including multiline input. Observe RED then GREEN.
- [x] Implement helper, configuration type, entrypoint validation and safe button error message. Seven targeted tests and vue-tsc passed.
- [ ] Independent read-only review; commit and integrate the verified source without discarding existing operations commits.
- [ ] Build immutable images through the existing GitHub CI. Do not build on production.
- [ ] Preserve current frontend image/config for rollback, add production frontend environment only, verify Compose differences and switch only frontend. Backend, Langflow, Eino, CRM, Wren and MCP remain running.
- [ ] Verify actual image ID and frontend commit, public button destination and local developer destination; verify anonymous Flow protection and original service health. Record separately that backend stays on its previous commit.

Deployment: LANGFLOW_EDITOR_URL is a frontend container environment value used to generate config.js at startup. Changing an env_file value alone requires recreating the frontend, not merely restarting it. An unset value keeps the previous local behavior. Only origins are accepted; credentials, query strings and fragments are not permitted.

Rollback: restore the previous frontend image and environment via the existing Compose service only; do not stop backend/runtime containers or delete any volumes.
