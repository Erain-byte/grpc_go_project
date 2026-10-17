# User 数据库迁移

`000001_initial_schema.up.sql` 创建：

- `user_identity_index`
- `user_groups`
- `user_group_members`
- `users_00` 至 `users_07`
- `user_wallets_00` 至 `user_wallets_07`
- `user_fund_transactions_00` 至 `user_fund_transactions_07`

生产环境应使用经过评审的版本化 SQL。本地开发可以继续使用 `go run ./cmd/migrate -f etc/user.yaml`。

使用 `psql` 执行：

```powershell
$env:PGPASSWORD = $env:USER_DB_PASSWORD
psql -h 127.0.0.1 -p 5434 -U user_service -d user_service `
  -v ON_ERROR_STOP=1 `
  -f migrations/000001_initial_schema.up.sql
Remove-Item Env:PGPASSWORD
```

回滚会删除所有 User 业务表及其中的数据，仅限空数据库或明确确认的数据重建场景：

```powershell
$env:PGPASSWORD = $env:USER_DB_PASSWORD
psql -h 127.0.0.1 -p 5434 -U user_service -d user_service `
  -v ON_ERROR_STOP=1 `
  -f migrations/000001_initial_schema.down.sql
Remove-Item Env:PGPASSWORD
```

