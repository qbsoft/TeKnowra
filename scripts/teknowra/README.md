# 生产版本一致性发布

适用 Windows + Docker Desktop 部署机。旧 watchtower 直接轮询 latest 会受加速器缓存、GHCR 超时影响，出现前后端版本不同、旧镜像回退和更新失败却汇总 Failed=0。

`Deploy-Release.ps1` 默认 Plan；Stage 准备所有镜像；Apply 在全部准备成功后用一次性 watchtower 更新，并核验全部容器。`Install-ReleaseTask.ps1` 安装每天 23:00 的计划任务，要求先有成功的 release-state.json。失败返回非零状态，日志和回滚快照在生产 `D:/deploy/watchtower/releases`。快照含环境变量，禁止提交。

首次迁移先备份旧 watchtower compose 与容器 inspect，再 Stage；停用旧调度后 Apply；成功后安装任务并实跑验证。现网已完成迁移；不要重新启动旧 watchtower。普通发版继续推原有 main/master 分支，由 GitHub 构建镜像，无需改流水线。

读取 GitHub Packages 使用原有只读包凭据，不需要仓库管理权限。平台通过成功 Actions 提交解析目标；私有 MCP/CRM 通过包的 SHA 标签及 digest 解析，CRM 前后端 SHA 必须一致。已验证版本时间用于拒绝向旧发布倒退。

本地离线测试：

```powershell
powershell -NoProfile -File .\Test-DeployRelease.ps1
```

运行时重建不是数据库或跨容器事务。出现 migration 时单独评估回滚；下载失败不会变更运行容器，重建失败则记录失败与快照，不掩盖成成功。网络仍不稳定时计划任务会重试，查询 LastTaskResult 和 deploy.log。

后端重建可能改变 Docker IP。前端 nginx 静态 upstream 会保留旧解析，因此后端就绪后必须 `nginx -t` 再平滑 reload 两个前端，并通过代理校验 OIDC 配置和 CRM health 的 JSON。容器自身 healthy、静态首页 200 都不足以证明登录正常。
