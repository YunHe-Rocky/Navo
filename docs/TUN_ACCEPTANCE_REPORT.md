# Navo TUN Acceptance Report

## Scope and result

- Result: Passed for the installed Navo 1.0.8 external-site usability scope.
- Executed: 2026-08-09 19:03-19:11 Asia/Shanghai.
- Windows: Microsoft Windows NT 10.0.26200.0.
- Package: `C:\Program Files\Navo` installed through Windows Installer.
- Profile: isolated clone of the current Navo profile; selected proxy credentials are not recorded.
- Scenario: `tun-off-tun`, including initial activation, 30-second delayed verification, disable, clean re-enable, and final rollback.

## Host environment

- Active physical egress: `以太网 2`, Remote NDIS based Internet Sharing Device.
- VMware VMnet1/VMnet2/VMnet8 adapters were present and preserved.
- Hyper-V: Disabled.
- Windows Subsystem for Linux: Disabled.
- Current-user WinINet proxy after acceptance: Disabled.

## Package integrity

| Item | Result |
|---|---|
| Installed version | 1.0.8 |
| Release manifest | Passed, 24/24 installed files matched |
| Installed repair check | Passed, 0 issues |
| MSI SHA-256 | `8f99a003c3f2868927e2f8dd03558cfb59221f8d13a28476a6b3a6b48db9ce88` |
| Setup SHA-256 | `57ad8916223ff37cce08f96f7f687ff36289fb40d1e6bc99f637d5931aec4946` |
| Wintun SHA-256 | `e5da8447dc2c320edc0fc52fa01885c103de8c118481f683643cacc3220dafce` |

## Installed data-plane matrix

| Core | Version | Initial | After 30 s | After re-enable | Exit IP | Rollback |
|---|---:|---|---|---|---|---|
| sing-box | 1.13.14 | Google 204, GitHub 200 | Google 204, GitHub 200 | Google 204, GitHub 200 | TUN/proxy `165.254.151.219` | Passed |
| Mihomo | 1.19.29 | Google 204, GitHub 200 | Google 204, GitHub 200 | Google 204, GitHub 200 | TUN/proxy `165.254.151.219` | Passed |

For every Service-owned stage, DNS, TCP, and HTTPS were true for both Google
and GitHub, and the stage was `HEALTH_COMMITTED`. Requests used no explicit
system or application proxy, so the Windows route/TUN data plane carried them.

## Network state and rollback

- Direct exit IP before each run: `211.90.237.75`.
- Direct exit IP after each run: `211.90.237.75`.
- Navo split routes before/after: 0 / 0.
- Navo NRPT rules before/after: 0 / 0.
- Navo firewall rules before/after: 0 / 0.
- Navo adapters before/after: 0 / 0.
- Package-owned processes after final acceptance: 0.
- Physical adapter DNS state was restored exactly by the acceptance snapshot comparison.

## Evidence

- `artifacts/tun-acceptance/20260809T110316676Z-mihomo-none-none-tun-off-tun.json`
- `artifacts/tun-acceptance/20260809T110735475Z-sing-box-none-none-tun-off-tun.json`

## Not covered by this follow-up

- Phase 34's full failure-injection and process-crash matrix was not rerun on
  Navo 1.0.8. Its older evidence remains historical and is not marked Passed
  in this report.
- The Windows Go race gate remains blocked because this host has no amd64
  GCC/Clang/MinGW CGO toolchain. Normal full tests and `go vet ./...` passed.
- UDP through the selected upstream was not claimed because that outbound did
  not declare UDP support; the verifier recorded it as unsupported rather than
  silently passing it.
