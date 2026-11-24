package payments

// pgx 读出的 JSONB 空值文本。仓储在多处用 `string(bytes) != "null"` 判定
// “列里没有 JSON 值”，集中命名避免各处拼写漂移。
const jsonNullLiteral = "null"
