# BENZHI 评测说明

基于 Go 实现的跨版本 API 语义弃用迁移证明后端服务，一款后端服务，完成契约版本登记、弃用字段语义转换与可见效果保持性比较、歧义路径检测与不可变迁移证明发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/apimigproof --addr :8080 --db apimigproof.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/apimigproof --smoke-test
```

`--smoke-test` 会真实创建契约版本、导入样本、执行歧义与兼容比较、发布迁移证明，关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出码结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/apimigproof --smoke-test
```

## HTTP API（前缀 /api）

契约：`POST/GET /api/contracts`、`GET /api/contracts/{id}`、`POST /api/contracts/{id}/seal`
字段：`POST/PUT/DELETE/GET /api/contracts/{id}/fields[/{field}]`
规则：`POST/GET /api/contracts/{id}/rules`、`DELETE /api/contracts/{id}/rules/{rid}`
样本：`POST /api/samples`、`POST /api/samples/batch`、`GET/DELETE /api/samples[/{id}]`
窗口：`POST/GET /api/windows`、`PUT /api/windows/{id}`
比较：`POST /api/compare`、`GET /api/comparisons[/{id}[/issues]]`
证明：`POST/GET /api/proofs[/{id}]`、`POST /api/proofs/{id}/publish`、`POST /api/proofs/{id}/supersede`
自检：`GET /api/health`、`POST /api/selfcheck`

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。契约、字段、规则、样本、比较结果与迁移证明在重启同一数据库后可恢复；封存契约不可修改。
