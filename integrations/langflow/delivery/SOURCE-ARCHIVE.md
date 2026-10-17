# M版对应源码归档及维护说明

`source-m-20261016.zip` 保存实际不可变编译阶段 `/app` 内的后端源文件、冻结的沙箱修复覆盖层、885项逐摘要匹配原构建记录的前端源文件、实际前端运行上下文（含已验收dist）、实际安装M组件payload及依赖锁/构建证据。不是将当前Git HEAD当作镜像来源。

`source-build-support.zip` 补充编译阶段实际公开CA证书（不含私钥）与正式运行镜像完整migration层的源文件/配方，逐文件摘要在SUPPORT-MANIFEST中。应用二进制指纹、两项产品修复文件摘要和实际镜像身份见M集合的RELEASE-MANIFEST。

## 如何定位与维护

1. 在可信来源核对本配套SHA256SUMS，再在新目录解压，分别读取SOURCE-MANIFEST和SUPPORT-MANIFEST。不要在现有工作区直接覆盖解压。
2. 后端的真实源树为 `backend-base/` **叠加** `backend-overrides/`。基础来自config ID `sha256:cad3106f583bda3724b045bbf40c94fed530e739a1d29699ba32cd3fc61b1bb6` 的实际编译阶段，包含当时生成的Go源；覆盖层来自10月16日冻结修复上下文。补入公开CA供Dockerfile COPY使用。只编译backend-base会漏掉本次修复。
3. 后端依赖由go.mod/go.sum、third_party以及scripts中的固定供应商版本/下载规则管理。原构建保留Anydoc/BrowserSkill，不能为了构建方便改成lite。按Makefile和Dockerfile设置Go/Rust工具链、准备Anydoc库，再运行 `VERSION=sandbox-restore-fix-20261016 COMMIT_ID=dirty-workspace-sandbox-fix make build-prod GO_BUILD_TAGS=anydoc`。正式维护改版应另起版本号，不覆盖M标签或归档。
4. `build/patch-candidate.Dockerfile` 是当时实际执行配方，包含测试阶段及本机构建器标签；`build/runtime-m.Dockerfile` 是去测试二进制后的M运行镜像配方。归档这些文件用于追溯，并不意味着接收机拥有这些本地标签。新构建环境应按相同源码/依赖重新构建各阶段，并固定基础镜像，不到生产服务器构建。
5. `runtime-build/complete-migrations/` 是正式运行镜像实际migration集合，不能用当前工作区后续DDL代替。与旧schema114升级的行为和库状态以验收记录为准，禁止据文件名推断已验证跨schema降级。
6. 前端源885项与原构建清单完全匹配。额外前端构建辅助文件若未包含在原清单，SOURCE-MANIFEST明确标为“current build-support”，不声称有当时指纹。已交付前端的精确静态产物来自 `frontend-runtime/dist`，可按原冻结Dockerfile重装运行镜像。源码重编前端仍需在隔离构建机按锁文件安装依赖并重新验证。
7. `langflow-m-payload` 是M运行组件的精确代码。**本轮新备份工具和部署模板**在本配套的tools/templates/docs/tests，不属于M镜像二进制的更改，不应声称它们已经在M镜像内。

归档包含完整的第一方编译源、依赖锁和指定运行构建输入；不打包Go/Rust/Node工具链、供应商下载缓存及镜像构建缓存，不承诺离线从源码构建，也未验证字节级可复现重编译。开源依赖及授权文件保留在原源树；外置CRM/MCP/模型是独立系统，版本和入口由dependency-inventory登记，不冒充随M镜像归档。

维护接手者可以保留本地源码快照或在自己的受控仓库登记新标签；本轮不替用户提交、推送或清理dirty工作区。
