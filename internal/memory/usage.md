# Memory subsystem (hot/warm/cold)

Amaros uses a **tiered memory model** to balance speed, durability, and long‑term archival.

## Tiers

1. **Hot** (in‑memory)
   - Fastest reads/writes
   - Stored in a Go map
   - Used for active/working state
2. **Warm** (append-only log files)
   - Durable across restarts
   - Stored as JSONL log files under `dataDir/warm/`
   - Last write wins when replaying the log
3. **Cold** (SQLite)
   - Long-term archival store
   - Stored in a file like `cold.db`
   - Good for history and querying by prefix

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
// Create a tiered store backed by disk.
tm, _ := memory.NewTieredMemory("/tmp/amaros", "/tmp/amaros/cold.db")
defer tm.Close()

// Write to hot
 tm.Set("foo", []byte("bar"), memory.Hot)

// Read (hot hit)
 e, _ := tm.Get("foo")

// Demote to warm
 tm.Demote("foo")

// Flush warm → cold
 tm.Flush()
```

## Notes

- `HotStore` is purely in‑memory; it is cleared on process exit.
- `WarmStore` is append‑only; `List` and `Get` will replay logs to find the latest value.
- `ColdStore` uses SQLite and supports prefix listing.
