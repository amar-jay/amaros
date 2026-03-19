
```go
provider := openrouter.New("sk-or-...")
resp, err := provider.Complete(ctx, model.CompletionRequest{
    Model:    "openai/gpt-4o",
    Messages: []model.Message{{Role: model.RoleUser, Content: "Hello"}},
})
```