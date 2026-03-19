## Core API

All tiers implement the same interface:
- `Get(key)`
- `Set(key, value)`
- `Delete(key)`
- `List(prefix)`
- `Close()`

### Tiered memory (recommended)

The `TieredMemory` type wraps all three tiers and provides:
- `Get(key)` that searches hot → warm → cold and auto‑promotes lower tiers to hot
- `Set(key, value, tier)` to write to a specific tier
- `Demote(key)` / `Promote(key)` to move a key between tiers
- `Flush()` to move all warm entries into cold
- `StartAging(ctx, hotTTL, warmTTL)` to auto‑demote entries over time

### Example usage

```go
tm, _ := memory.NewTieredMemory("/tmp/amaros", "/tmp/amaros/cold.db")
defer tm.Close()

 tm.Set("foo", []byte("bar"), memory.Hot)

 e, _ := tm.Get("foo")

// Demote to warm
 tm.Demote("foo")

// Flush warm → cold
 tm.Flush()
```