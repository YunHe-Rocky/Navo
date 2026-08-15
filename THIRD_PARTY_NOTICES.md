# Third-party notices

Navo distributes unmodified third-party runtime components. These projects are
independent from Navo; inclusion does not imply endorsement or association.

| Component | Bundled version | License | License file | Upstream |
|---|---:|---|---|---|
| sing-box | 1.13.14 | GPL-3.0-or-later plus upstream name restriction | `third_party/sing-box/LICENSE` | https://github.com/SagerNet/sing-box |
| Mihomo | 1.19.29 | GPL-3.0 | `third_party/mihomo/LICENSE` | https://github.com/MetaCubeX/mihomo |
| Xray-core | 26.3.27 | MPL-2.0 | `third_party/xray/LICENSE` | https://github.com/XTLS/Xray-core |
| Wintun | bundled DLL | Wintun prebuilt binaries license | `third_party/wintun/LICENSE.txt` | https://www.wintun.net/ |

The exact executable versions and SHA-256 digests used by a Navo build are
recorded in `CORE_MANIFEST.json`. The release package also contains
`SHA256SUMS.txt`, which covers every immutable packaged file.

Navo's own source code is currently all-rights-reserved because the repository
does not yet contain a project-level `LICENSE` file. Third-party licenses apply
to their respective components regardless of Navo's project-level status.

Before replacing any bundled component:

1. Review the new upstream license and notices.
2. Update the component version and SHA-256 in `CORE_MANIFEST.json`.
3. Preserve the corresponding license files in the portable package.
4. Rebuild and verify the closed package manifest.
