# HANDOFF — read this fully before any work

Generated: 2026-09-20T08:07:24Z · Updated: 2026-09-20T17:00Z · Git HEAD: no-git
Trust rule: `[V]` = verified in the current repository/deployment checks. `[?]` = unresolved or not rechecked.

## 1. Current state

[V] The directory is not a Git repository; `git status`/`git log` report “not a git repository”.
[V] Main panel is deployed on `156.239.227.123` with `gost-mysql`, `springboot-backend`, and `vite-frontend`; unrelated `sub2api`, PostgreSQL/Redis, and `certimate` services were preserved.
[V] MySQL data was intentionally reset earlier at the user’s request; subsequent application updates used `docker compose up -d --no-deps backend frontend` and did not touch the MySQL volume.
[V] The custom public-port feature is now deployed in the panel Docker images. Latest backend/frontend images were built successfully and containers reached `backend=true`, `frontend=true`, `mysql=healthy`, `http=200`.
[V] Current database counts after deployment were checked as `user=2`, `inbound=8`, `forward=9`; no data reset occurred during the port-feature update.
[V] Frontend `npm run build` passes; only the existing Vite dynamic/static import warning is emitted.

## 2. Custom public-port feature

[V] The requested `port: 20000` is the public gost `Forward.inPort`, not `InboundDto.listenPort` (the internal sing-box `127.0.0.1:40000+` port).
[V] `InboundUserDto` has nullable validated `publicPort` (transient request field; no schema migration).
[V] Direct and relay assignment forms in `vite-frontend/src/pages/inbound.tsx` and `relay.tsx` expose “公网端口起始值(可空自动分配)”.
[V] Backend assignment flow passes the value through `assignUser`, `assignAllToUser`, and `assignOneNoPush` into `createForwardAutoPort`.
[V] Blank input preserves automatic allocation. A single protocol uses the exact requested port; batch assignment uses base, base+1, … for newly created forwards. Existing forward ports are unchanged.
[V] Explicit range/DB/OS conflicts do not silently switch ports. Batch-created records are rolled back when an explicit-port allocation fails.
[V] Clash and normal subscriptions already serialize `Forward.inPort`; no subscription/Clash changes are required for this feature.
[V] Main panel Docker was updated with the feature; refresh the panel before testing. No real custom-port acceptance test has yet been recorded.

## 3. Node/startup state

[V] Local `install.sh` supports a `LOCAL_GOST_BINARY` override, systemd/OpenRC/SysV registration, supervisor generation, and Docker entrypoint generation.
[V] `go-gost/x/socket/singbox.go` has OpenRC/SysV support, correct sing-box command arguments, supervisor cwd handling, and stop→start behavior on config reload.
[V] `go-gost/entrypoint.sh` keeps PID ownership consistent and avoids duplicate sing-box supervisors.
[V] Target nodes `142.91.99.218:51811` and `199.30.88.156:21709` were installed with the local installer plus the locally built Linux gost; both use OpenRC and have connected to the panel.
[V] The two-node relay issue was caused by OpenRC `start` not reloading already-running sing-box. Both nodes were manually restarted with `rc-service sing-box restart`; relay inbounds then loaded.
[V] A final reload-safe gost binary was built remotely from local source and deployed to both nodes. Old binaries remain as `gost.before-reload-fix` for rollback.
[V] For the relay test, public TCP ports were reachable and target sing-box configurations contained the configured users/Reality values; historical errors were from stale sing-box processes before the restart.
[?] A fresh end-user Clash connection after the final restart has not been independently verified; user should refresh the `/clash` subscription and test.

## 4. Important deployment facts

[V] Public GitHub `releases/latest` assets may still be older than local fixes. The current nodes were fixed by uploading local installer/binary assets, not by the remote curl installer.
[V] A clean new node install should use the local `install.sh` with `LOCAL_GOST_BINARY=/path/to/gost-amd64`; do not assume `releases/latest` contains the fix until explicitly published.
[V] Main panel’s current Docker update path preserves MySQL: build backend/frontend, then `docker compose up -d --no-deps backend frontend`.
[V] The test container `tms-linux-chicken-128m` was intentionally removed; do not assume it exists.

## 5. Verification limitations

[V] Frontend build passes.
[V] Remote Linux gost builds from local source have completed successfully.
[?] A final Maven command result for the latest local source was not captured independently, although the Docker backend build that deployed the current image completed successfully after fixing the log API compile error.
[?] No real end-user proxy/Clash acceptance test has been captured; protocol traffic remains the user’s next test.

## 6. Next steps

1. Test the deployed custom port: assign one direct and one relay protocol with blank port and with a custom base; verify `Forward.in_port`, TCP/UDP gost listeners, ordinary/Clash subscription ports, and collision errors.
2. Refresh the Clash subscription after the final sing-box restart and test both relay nodes.
3. If fresh installs must use the original public curl command, explicitly authorize publishing the rebuilt install script/gost assets to GitHub releases; until then the public latest asset remains a trap.
4. Keep the OpenRC service fix in future gost builds; do not manually rely on `rc-service sing-box start` after config changes—use the reload-safe binary/service path.

## 7. Known traps

- Do not add public port to `InboundDto.listenPort`; it is the local sing-box port.
- Do not share one public port across multiple protocol forwards; batch custom input is a starting port.
- Do not run `docker compose down -v` unless database deletion is explicitly requested.
- Do not infer public reachability only from a same-provider server; test from a genuinely external network when investigating NAT/security groups.
- Local Windows Go builds may fail on dependency downloads; use the remote Linux Go Docker build.
