## Memory Tiers (Basic)

- Hot: Fast, active working memory for the current task. In this project it uses Chroma-backed semantic retrieval, so queries can return the most relevant memory even when text is not an exact match.
- Warm: Intermediate, append-only log storage for recent but less-active context. Good for short-term history and task traces.
- Cold: Durable long-term archive in SQLite for historical data and recovery.

Think of the flow as:
- Hot = what the agent is thinking about now
- Warm = what the agent did recently
- Cold = what the agent should remember over time

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
tm, _ := memory.NewTieredMemory("/tmp/amaros")
defer tm.Close()

tm.Set("foo", []byte("bar"), memory.Hot)

e, _ := tm.Get("foo")

// Demote to warm
tm.Demote("foo")

// Flush warm → cold
tm.Flush()
```