# Silo

Silo 是一个致力于提供高性能、工业级 Go 语言工具库的集合。它充分利用 Go 泛型特性，旨在为日常开发提供零依赖、高效且健壮的工具函数。

## 📦 模块列表 (Modules)

目前已实现的模块：

*   **sliceutil**: 高性能切片操作工具库，包含并发处理、集合操作与原地过滤等特性。
*   *(更多模块如 `maputil`, `strutil` 等正在规划中...)*

## 🛠️ 安装 (Installation)

```bash
go get silo/sliceutil
```

> **注意**: 需要 Go 1.21+ (使用了 `min` 内置函数及泛型)。

---

## 🔪 sliceutil

`sliceutil` 是 Silo 的切片工具包，在性能优化（BCE、内存预分配、零分配原地操作）和并发控制方面达到了生产级标准。

### ✨ 核心特性

*   **🚀 极致性能**: 广泛应用 **BCE (边界检查消除)** 技术和启发式内存预分配，最大程度减少运行时开销。
*   **♻️ 零分配 (Zero Alloc)**: 提供 `FilterInPlace` 等原地修改函数，并在缩容时自动清理尾部指针，对 GC 极其友好。
*   **⚡ 智能并发**: `TryParallelMap` 和 `TryParallelFilter` 自动根据数据量决定是否启用并发。支持 `context.Context` 取消、Fail-Fast（快速失败）机制，且采用无锁聚合策略。
*   **🛡️ 健壮的错误处理**: 所有核心函数均提供 `Try*` 版本（如 `TryMap`, `TryFilter`），完美支持可能返回错误的业务逻辑。
*   **🧩 集合与查找**: 提供高效的 `Union` (去重并集)、`Intersection` (交集) 以及 `Find`、`Contains` 等常用工具。

### 📚 API 概览

| 类别 | 函数 | 说明 |
|---|---|---|
| **集合** | `Intersection`, `IntersectionBy` | 交集 |
| | `Union`, `UnionBy` | 并集 |
| | `Difference`, `DifferenceBy` | 差集 |
| | `SymmetricDifference`, `SymmetricDifferenceBy` | 对称差集 |
| | `Unique`, `UniqueBy` | 原地去重 |
| **转换** | `Map`, `TryMap` | 映射 |
| | `Filter`, `TryFilter` | 过滤 |
| | `FilterInPlace`, `TryFilterInPlace` | 原地过滤 (零分配) |
| | `Reduce`, `TryReduce` | 归约 |
| **并发** | `TryParallelMap`, `TryParallelMapWithContext` | 并发映射 |
| | `TryParallelFilter`, `TryParallelFilterWithContext` | 并发过滤 |
| **查找** | `Contains`, `ContainsFunc` | 检查元素是否存在 |
| | `Find`, `FindIndex` | 查找元素或索引 |

---

## 🧪 测试 (Testing)

运行测试以确保所有功能正常：

```bash
go test ./sliceutil
```

运行带详细输出的测试：

```bash
go test -v ./sliceutil
```

---

## 🤝 贡献 (Contributing)

欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

请确保所有测试通过，并且代码符合项目的风格。

---

## 📄 许可证 (License)

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

---

## 📝 Git Commit 规范 (Convention)

本项目严格采用 **[Conventional Commits](https://www.conventionalcommits.org/)** 规范。为了保持 Commit Log 的整洁和可读性，请务必遵守以下格式。

### 格式 (Format)

```text
<type>(<scope>): <subject>
// 空一行
[body]