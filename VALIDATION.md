# 验收记录

- 日期：2026-08-22
- 静态检查：Go 1.22 下 `go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./...` 通过；Vue 类型检查和 Vite 生产构建通过；`docker compose config --quiet` 通过。
- 容器启动：PostgreSQL、Redis、backend、frontend 从空数据卷启动成功并达到 healthy，`GET /healthz` 返回 200，后端运行日志未见异常。
- API 流程：管理员登录、概览、4 个实体列表、创建靠泊任务、合法状态迁移、会话、脱敏运行配置、审计列表及审计汇总均通过。
- RBAC 与双人确认：viewer 写操作返回 403；operator 首次提交后许可保持 `pending`；提交人自审返回 422；不同账号的 reviewer 完成放行。审计保存窗口版本、操作者和请求 ID。
- 窗口关联与失效（2026-09-25 新增）：窗口新增 `expireAt`（须晚于 `effectiveAt`）；许可创建/提交按「区域+窗口编码」重新读取 safe 窗口并固化窗口版本与有效期；复核放行前再次读取，换版（422 并作废提交结论、保持 pending）、受限/过期（许可置 restricted/expired 并写原因）均拒绝放行；窗口 restricted/expired 迁移级联使同区域 pending 许可失效；受限期间重新提交被 422；同编码不同区域窗口互不影响。以上由 `service` 单元测试与端到端 HTTP 脚本（27 项检查全部 PASS）覆盖。
- 内置 Browser：验证船舶靠泊、系泊方案、风浪窗口、安全许可、审计记录 5 个页面；`RiskBadge` 与跨页 `ClearancePanel` 正常展示；实际以 admin 复核 operator 已提交的 `SC-001`，状态刷新为 `cleared`，审计回显窗口 v1 与请求 ID；控制台 0 error / 0 warning，桌面截图未见遮挡或错位。
- 规模：3095 行 Go 功能代码，38 个非测试 `.go` 文件。
- 清理：验收完成后执行 `docker compose down -v --remove-orphans`，清除本项目容器和数据卷。
