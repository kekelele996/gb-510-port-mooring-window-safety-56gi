请生成 `port-mooring-window-safety`「港口系泊安全窗口评估」Go 全栈项目，面向港口运营方评估船舶系泊方案、风浪窗口、缆绳检查和靠泊安全许可。不要实现售票、预约、订单、库存或财务结算。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`VesselCall`（船舶靠泊任务）、`MooringPlan`（系泊方案）、`WeatherWindow`（风浪窗口）、`SafetyClearance`（许可决定）全链路贯穿数据库、Go model/service/handler、前端 API/store/page。

### 核心页面

`/vessels` 船舶与泊位信息；`/plans` 系泊方案；`/weather-windows` 安全窗口；`/clearance` 许可审核；`/audit` 审计。`RiskBadge` 在方案和窗口页共用，`ClearancePanel` 在窗口和许可页共用。

### 横切关注点

RBAC + 双人安全确认同步 DB 角色、Go middleware、路由守卫、前端按钮；许可变更审计保存窗口版本、操作者和请求 ID；全局错误处理和限流必须跨层。

### 共享枚举/组件

同步 `CallState`（planned/approach/moored/departed）与 `ClearanceState`（pending/cleared/restricted/expired）。共享 `StatusBadge`、`RiskBadge`、`ConfirmDialog`，hooks 为 `useAuth`、`usePagination`。

### 技术与规模要求

前端 Vue 3 + TypeScript + Vite + Element Plus；后端 Go 1.22 + Gin + GORM；PostgreSQL + Redis。目标 3000–4200 行、30–42 个 `.go` 文件。

### 文件结构强制清单

前端 `api/stores/types/components/common/hooks/pages/router/utils`，后端 `model/dto/repository/service/handler/router/middleware/constants/util`，README 列出共享枚举所有出现位置。

### 结构红线

严禁合并职责到单一文件；窗口、方案和许可改动必须跨多层协同。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: port-mooring-window-safety`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=port-mooring-window-safety`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18510`、后端端口 `19510`；前端用 `/api` + Nginx，数据库健康检查、命名卷和 `condition: service_healthy` 齐全，提供真实 `/healthz` 和 Git 初始化。
