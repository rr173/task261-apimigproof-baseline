# task261-apimigproof · 跨版本 API 语义弃用迁移证明服务

基于 REQ-20260825-123 生成。平台工程师登记契约版本与字段语义、导入旧客户端请求样本后，
服务构造跨版本转换路径并比较**可见效果**（在场性 + 值），报告丢失区分度、冲突默认值等
歧义路径；工程师声明兼容窗口、确认拒绝策略并发布绑定新旧契约与样本证据的不可变迁移证明。

## 业务闭环

1. 登记契约版本（名称 + 单调递增版本号）与字段语义（有效/废弃/转换/冲突）；
2. 为废弃字段注册转换规则（保留/改名/强转/补默认/丢弃/拒绝），拒绝转换循环；
3. 导入请求样本（内容指纹幂等，重复导入只保留一条）；
4. 执行语义保持性比较：逐字段比较跨版本可见效果，产出可迁移/被拒绝/语义改变；
5. 声明兼容窗口（保留/转换/拒绝策略）并调整规则后复跑比较；
6. 基于兼容比较发布迁移证明（绑定契约对 + 比较 + 证据指纹，封存后不可修改，只能被新证明替代）。

## 核心状态机

- 契约版本：`draft → pending_compare → compatible | ambiguous → sealed`（sealed 终态）
- 字段语义：`valid → deprecated → transformed | conflict`
- 请求样本：`original → migratable | rejected | semantics_changed`
- 迁移证明：`draft → published → superseded`

## 标准命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/apimigproof --smoke-test
go run ./cmd/apimigproof --addr :8080 --db ./apimigproof.db
```

## API 入口

全部为 `/api` 前缀的 JSON 接口：契约与字段（`/api/contracts[/{id}/fields[/{field}]]`）、
封存（`/seal`）、转换规则（`/api/contracts/{id}/rules`）、样本（`/api/samples[/batch]`）、
兼容窗口（`/api/windows`）、比较（`/api/compare`、`/api/comparisons[/{id}[/issues]]`）、
证明（`/api/proofs[/{id}[/publish|supersede]]`）、自检（`/api/health`、`/api/selfcheck`）。

## 持久化与重启恢复

SQLite（`modernc.org/sqlite`，纯 Go）保存契约、字段语义、转换规则、样本、兼容窗口、
比较任务与证明。`--smoke-test` 会关闭并重开同一数据库，断言全部实体与状态恢复，
封存契约恢复后仍拒绝修改。样本指纹唯一约束保证幂等；同一契约对同时只允许一个
running 比较（串行约束）。

## 组件版本

- Go 1.26.3（`GOTOOLCHAIN=local`，CGO_ENABLED=0）
- SQLite 3.46.1（`modernc.org/sqlite` v1.52.0）
- 详见 `component-versions.json`
