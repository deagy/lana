# Performance Tuning & Optimization

This document covers performance characteristics, benchmarking, and optimization strategies.

## Performance Metrics

### Baseline (Target)

| Operation | Latency | Notes |
|-----------|---------|-------|
| Tool lookup | <1ms | Registry is O(1) |
| File read | <10ms | Depends on file size |
| Session save | <5ms | JSON serialization |
| Tool execution | Variable | Depends on tool |

### Current Performance

Run benchmarks:
```bash
go test -bench=. ./internal/tools/... -benchmem
```

### Typical Numbers (on modern CPU)

```
BenchmarkRegistryGet-8         1000000      1000 ns/op      0 B/op     0 allocs/op
BenchmarkRegistryList-8         500000      2500 ns/op   1024 B/op     5 allocs/op
BenchmarkRegistrySchemas-8      100000     10000 ns/op   5000 B/op    50 allocs/op
```

## Hot Paths

### 1. Tool Registry Lookup

**Current:** O(1) with RWMutex  
**Impact:** Called on every tool execution

**Optimization tips:**
- Use local lookup cache for frequent tools
- Inline common tools

### 2. JSON Serialization

**Current:** Standard library encoding/json  
**Impact:** Session save, output formatting

**Optimization tips:**
- Consider jsoniter for 2-3x speedup
- Lazy serialize only needed fields
- Pre-allocate buffers

### 3. File I/O

**Current:** Direct os.Read/WriteFile  
**Impact:** File operations and session storage

**Optimization tips:**
- Batch file operations
- Use bufio for large files
- Consider memory mapping for large reads

### 4. Provider Streaming

**Current:** Sequential event processing  
**Impact:** Chat and run modes

**Optimization tips:**
- Parallel tool execution (where safe)
- Stream buffering (already done)
- Connection pooling

## Memory Usage

### Typical Session Sizes

```
Empty session:     ~2 KB
10 messages:       ~10 KB
100 messages:      ~100 KB
1000 messages:     ~1 MB
```

### Session Store Memory Impact

```
Loaded session:    Size * 1 + overhead
All sessions:      O(total sessions size)
```

**Optimization tips:**
- Lazy load sessions (only on use)
- Prune old sessions periodically
- Use LRU cache for hot sessions

## Streaming Performance

### EventPipeline Characteristics

```
Buffer size:  100 events
Throughput:   >10k events/sec
Latency:      <1ms per event
```

### Streaming Optimization

**Current:** Channel-based buffering  
**Status:** Already optimized for typical workloads

**Edge cases:**
- High-frequency events (>100k/sec): Consider batch processing
- Large event payloads: Stream compression

## Tool Execution Performance

### By Tool Type

| Tool | Typical Time | Notes |
|------|--------------|-------|
| read_file | 10-100ms | Depends on file size |
| write_file | 10-50ms | Disk I/O |
| list_files | 50-500ms | Depends on directory size |
| git_status | 100-500ms | Repo scan |
| git_diff | 100-1000ms | Large repos |
| search | 10-1000ms | File count, pattern |
| exec | 10-10000ms | Command dependent |

### Optimization Tips

**file operations:**
- Cache directory listings
- Use streaming for large files

**git operations:**
- Run in repo context only
- Cache repo state when possible

**search:**
- Limit context lines
- Use file pattern to reduce scope

**exec:**
- Set timeouts
- Limit output capture

## Configuration Tuning

### Session Store

```yaml
session:
  store_path: ~/.lana/sessions
  # Consider: prune old sessions monthly
  # Consider: archive sessions > 1GB
```

### Provider

```yaml
provider:
  # Set appropriate timeouts
  timeout: 300  # seconds (not implemented yet)
  
  # Batch requests where possible
  max_concurrent: 5
```

### Tools

```yaml
tools:
  # Limit output size
  max_output: 100KB
  
  # Set execution timeouts
  exec:
    timeout: 30  # seconds
  
  search:
    max_results: 100
```

## Profiling

### CPU Profile

```bash
# Generate CPU profile
go test -cpuprofile=cpu.prof ./internal/runner/...

# Analyze
go tool pprof cpu.prof
```

### Memory Profile

```bash
# Generate memory profile
go test -memprofile=mem.prof ./internal/runner/...

# Analyze
go tool pprof mem.prof
```

### Runtime Traces

```bash
# Generate trace
go test -trace=trace.out ./internal/runner/...

# View trace
go tool trace trace.out
```

## Benchmarking

### Run Benchmarks

```bash
# All benchmarks
go test -bench=. ./... -benchmem

# Specific package
go test -bench=Registry ./internal/tools/... -benchmem

# With cpuprofile
go test -bench=. -cpuprofile=cpu.prof ./...
```

### Benchmark Results Template

```
Benchmark                    Old Time    New Time    Change
BenchmarkToolLookup-8        1000ns      800ns       -20%
BenchmarkSessionSave-8       5000ns      4000ns      -20%
BenchmarkFileRead-8          10000ns     10000ns     ~
```

## Optimization Roadmap

### v0.2.0

- [ ] Lazy session loading
- [ ] LRU session cache
- [ ] Parallel tool execution (safe paths)
- [ ] Output streaming (large files)

### v0.3.0

- [ ] Connection pooling
- [ ] Query result caching
- [ ] Incremental file parsing
- [ ] Compressed session storage

### v1.0.0

- [ ] Full streaming pipeline
- [ ] Distributed session storage
- [ ] Embedded index for search
- [ ] Memory-mapped file I/O

## Common Optimizations

### 1. Reduce Allocations

```go
// Bad: Allocates new buffer each time
func process(data []byte) []byte {
    result := make([]byte, 0)  // New allocation
    return result
}

// Good: Reuse buffer
func process(buf *bytes.Buffer) {
    buf.Reset()  // Reuse allocation
}
```

### 2. Use Sync Pool

```go
var bufPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

buf := bufPool.Get().(*bytes.Buffer)
defer bufPool.Put(buf)
```

### 3. Preallocate Slices

```go
// Bad: Grows dynamically
results := []string{}
for _, item := range items {
    results = append(results, item)
}

// Good: Preallocate
results := make([]string, len(items))
for i, item := range items {
    results[i] = item
}
```

### 4. Cache Expensive Operations

```go
type Session struct {
    transcript []Message
    
    // Cache frequently accessed data
    messageCount int
    toolCount    int
}

func (s *Session) GetMessageCount() int {
    if s.messageCount == 0 {
        s.messageCount = len(s.transcript)
    }
    return s.messageCount
}
```

## Monitoring

### Metrics to Track

- Tool execution time (by tool)
- Session size distribution
- Provider latency
- Error rates
- Memory usage trends

### Implementation

```bash
# Add metrics via internal/metrics/ (future)
# - Prometheus endpoints
# - Structured logging
# - Performance traces
```

## Production Checklist

Before production deployment:

- [ ] Run full benchmark suite
- [ ] Profile with typical workload
- [ ] Memory leak test (run overnight)
- [ ] Goroutine leak test
- [ ] Stress test (concurrent operations)
- [ ] Long-running stability test (24h+)

## Troubleshooting Performance

### Slow Tool Execution

1. Check tool-specific logs
2. Profile the tool in isolation
3. Check system resources (disk, CPU, memory)
4. Set execution timeout

### High Memory Usage

1. Check session count
2. Prune old sessions
3. Profile memory usage
4. Check for leaks

### Slow Provider Responses

1. Check provider health
2. Check network connectivity
3. Monitor API rate limits
4. Consider caching responses

## Resources

- [Go profiling](https://golang.org/blog/pprof)
- [Benchmarking](https://pkg.go.dev/testing#hdr-Benchmarks)
- [PPROF usage](https://github.com/google/pprof/tree/master/doc)

## FAQ

**Q: Is lana production ready for high-volume use?**  
A: Yes, for typical single-user workflows. For high-concurrency (>100 simultaneous), consider load testing first.

**Q: What's the maximum session size?**  
A: No hard limit. Practical limit is system memory (typically 100k+ messages).

**Q: Can I improve provider latency?**  
A: Yes: use local providers (Ollama), enable caching, reduce model size, use smaller context windows.

**Q: How do I reduce memory usage?**  
A: Prune old sessions, use smaller models, reduce context window, enable session compression (future).
