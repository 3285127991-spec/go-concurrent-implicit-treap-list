# go-concurrent-implicit-treap-list

基于 Implicit Treap（隐式笛卡尔树）的并发安全 string 列表。纯 Go 标准库实现，无第三方依赖，无持久化 / 网络 / GUI。

## 特性

- 每个节点维护子树 size，按下标的操作全部通过 split / merge 完成，期望 O(log n)
- Reverse 只翻转目标子树根上的 lazy 标记，不逐项交换，标记在 split / merge 时下传
- 节点优先级 = `mix(seq)`：`seq` 为严格递增的内部序号，`mix` 是固定的 splitmix64 确定性混合函数，相同操作序列必然得到相同树形（可复现）
- `sync.RWMutex` 保护：Get / Snapshot / Len 走读锁且不下传 lazy 标记（不修改树），多 goroutine 并发读写安全
- Snapshot 为 O(n) 独立副本；Close 幂等，关闭后拒绝写操作、保留读操作

## API

| 方法 | 返回 | 说明 |
| --- | --- | --- |
| `New()` | `*List` | 创建空列表 |
| `Insert(index, values)` | `error` | 在 `index` 前插入非空 values，要求 `0 <= index <= Len` |
| `Delete(left, right)` | `error` | 删除半开区间 `[left, right)`，要求 `0 <= left < right <= Len` |
| `Reverse(left, right)` | `error` | 反转半开区间 `[left, right)`，同样的区间约束 |
| `Get(index)` | `(string, error)` | 读取下标元素，要求 `0 <= index < Len` |
| `Snapshot()` | `[]string` | 返回当前序列的独立副本 |
| `Len()` | `int` | 元素个数 |
| `Close()` | | 幂等关闭；之后 Insert/Delete/Reverse 返回 `ErrClosed` |

错误哨兵：`ErrIndexOutOfRange`、`ErrInvalidRange`、`ErrEmptyValues`、`ErrClosed`，可用 `errors.Is` 判断。

## 示例

```go
l := implicittreaplist.New()
l.Insert(0, []string{"a", "b", "c", "d"})
l.Insert(2, []string{"x", "y"}) // [a b x y c d]
l.Reverse(1, 5)                 // [a y x b c d]
l.Delete(0, 2)                  // [x b c d]
fmt.Println(l.Snapshot(), l.Len())
l.Close()                       // 之后 Get/Snapshot/Len 仍可用
```

## 运行 CLI 演示

```sh
go run ./cmd/treapcli
```

演示中间插入、区间 Reverse、Delete，以及 Close 后读取与写操作被拒绝。

## 测试

```sh
go test -race ./...
```

8 个测试覆盖：头/尾/中间插入、Delete、Reverse、连续 lazy 混合操作（对照朴素切片模型）、边界错误、Snapshot 独立性与 Close 语义、基础并发读写、结构可复现性。
