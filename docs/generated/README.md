# docs/generated/ — 机器生成渲染层

- **定位**：本目录全部文件由生成器产出，是**渲染层（机器生成）**，**禁止手改**。
- **事实源**：注册表渲染 md（`*-registry.md`、`security-readiness.md`）唯一事实源为
  `contracts/registries/*.yaml`。
- **修改方式**：先改 YAML，再运行 `python3 scripts/gen_registry_md.py` 重新生成；手改会在
  零差异门禁中失败。
- **门禁**：`python3 scripts/gen_registry_md.py --check`（由 `bash scripts/check-generated-zero-diff.sh` 串联）。
