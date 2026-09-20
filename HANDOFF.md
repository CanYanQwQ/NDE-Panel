# HANDOFF — read this fully before any work

Generated: 2026-09-20T08:07:24Z · Updated: 2026-09-20T20:00Z · Git HEAD: 65cc52e
Trust rule: `[V]` = verified in the current repository/deployment checks. `[?]` = unresolved or not rechecked.

## 1. Current state

[V] The project is now a Git repository on branch `main`, remote `origin` is `https://github.com/CanYanQwQ/NDE-Panel.git`, and the working tree is clean.
[V] Remote `main` points to commit `65cc52e4a00e642a3de53c25dfb4bc56a7475b6d`.
[V] GitHub release `gost-v1.0.2` is published and not draft/prerelease; assets verified through the GitHub API: `gost-amd64`, `gost-arm64`, `install.sh`.
[V] `releases/latest/download/install.sh` and `releases/latest/download/gost-amd64` were downloaded successfully; the release installer contains the new repository URL and `LOCAL_GOST_BINARY` support.
[V] Docker workflow for the new repository completed successfully earlier; project GHCR references use `ghcr.io/canyanqwq/...`.

## 2. Current local feature state

[V] Custom public port is implemented and deployed:
- `InboundUserDto.publicPort` is nullable/validated and transient.
- `InboundServiceImpl` passes it through direct, batch, and relay assignment.
- Blank keeps automatic allocation; a single assignment uses the exact port; batch uses base, base+1, … for new forwards.
- Explicit conflicts/range/OS-bind errors do not silently change ports; batch-created records roll back on explicit failure.
- `inbound.tsx` and `relay.tsx` show “公网端口起始值(可空自动分配)”.
- Existing forward ports, limiter, flow, expiry, relay landing, subscription, and sing-box local ports are preserved.
[V] Node warning suppression is implemented: backend returns transient `inboundCount`, and frontend only shows sing-box failure when a node has protocols.
[V] `npm run build` passes in the current handoff run; only the existing Vite dynamic/static import warning appears.
[?] Full Maven compile/test for the newest local custom-port source was not independently captured in the current handoff run; the Docker backend image containing the feature was successfully built earlier after fixing the log API compile error.

## 3. Remote deployment state

[V] Main panel `156.239.227.123` runs `gost-mysql`, `springboot-backend`, and `vite-frontend`; latest application update used `up -d --no-deps backend frontend` and preserved MySQL data. Other services were preserved.
[V] Main panel custom-port image deployment completed with backend/frontend ready and HTTP 200. Current database counts after user activity were checked as `user=2`, `inbound=8`, `forward=9`.
[V] Test container `tms-linux-chicken-128m` was deleted as requested.
[V] Nodes `142.91.99.218:51811` and `199.30.88.156:21709` use local installer/new gost and OpenRC. The relay issue was fixed by correcting OpenRC sing-box command arguments, supervisor cwd, and stop→start reload behavior.
[V] Both target nodes were reloaded with current sing-box configuration; node/relay services and public ports were verified during diagnosis. Historical invalid UUID/Reality errors came from stale sing-box processes before reload.

## 4. GitHub migration and release

[V] Project-owned old repository URLs were migrated to `CanYanQwQ/NDE-Panel`, including installer/raw/release URLs, README references, GitHub API version checks, Compose source/image references, and workflow GHCR tags.
[V] Third-party URLs and upstream attribution were retained.
[V] Root `.gitignore` excludes environment files, node_modules, build outputs, targets, backups, Claude files, and temporary artifacts.
[V] Tracked environment files are only frontend development/production files; they contain no high-confidence secrets. A scan found no API keys, GitHub tokens, private-key material, or server credentials. The source still contains the intentional default `admin_user/admin_user` credential and must be changed after deployment.
[V] No force-push was used; remote was empty before initial push.

## 5. Known traps

- Public port is `Forward.inPort`, not `Inbound.listenPort`; custom input belongs to assignment modals.
- Batch custom port is a starting value, not one shared port.
- The public `latest` release is now updated, but a clean install still needs the release asset and correct panel address/secret.
- OpenRC sing-box configuration changes must use the reload-safe gost binary; a plain `rc-service sing-box start` while already running may not reload old configuration.
- Do not run `docker compose down -v` unless database deletion is explicitly requested.
- For NAT/container nodes, public TCP/UDP port forwarding and security-group rules must be tested from an external network.

## 6. Next steps

1. Perform real end-user tests for direct and relay Clash subscriptions.
2. Test custom public port with blank input, a single exact port, a batch base, out-of-range values, and occupied ports; verify `Forward.in_port`, gost listeners, and subscription output.
3. Rotate the default admin password after deployment.
4. If any release/workflow change is needed, update the repository and publish a new tag rather than modifying server assets manually.
