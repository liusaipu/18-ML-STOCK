# Contributing to StockFinLens

感谢你对 **股票财报透镜 (StockFinLens)** 项目的关注！以下是参与贡献的指南。

---

## 开发环境搭建

1. 克隆仓库
   ```bash
   git clone https://github.com/liusaipu/stockfinlens.git
   cd stockfinlens
   ```

2. 安装依赖
   - Go >= 1.22
   - Node.js >= 18
   - Python >= 3.10
   - Wails CLI >= v2.12

   详见 [docs/BUILD.md](docs/BUILD.md)。

---

## 代码规范

### Go
- 遵循标准 Go 代码风格，提交前运行 `go fmt ./...`
- 优先使用项目已有的错误处理方式
- 涉及并发操作时，务必添加 `recover()` 防止 panic 扩散

### TypeScript / React
- 前端使用 React + TypeScript，确保 `npm run build` 无类型错误
- ECharts 数据处理时避免传入 `null`，统一使用 `'-'` 或 `undefined`

### Python
- ML 脚本位于 `ml_models/` 和 `scripts/`，保持与现有推理接口兼容
- ONNX 模型导出后请验证与 Go 端的推理结果一致

---

## 提交规范

Commit message 建议遵循以下前缀：

| 前缀 | 用途 |
|------|------|
| `feat:` | 新功能 |
| `fix:` | 修复 bug |
| `docs:` | 文档更新 |
| `ui:` | 界面/交互优化 |
| `chore:` | 构建/版本/工具链等杂项 |
| `refactor:` | 重构（无功能变化） |

示例：
```
fix: 修复 Windows 删除报告对话框返回值兼容问题
feat: 添加联动图表双击全屏功能
```

---

## Pull Request 流程

1. 从 `main` 分支创建你的功能分支：`git checkout -b feature/xxx`
2. 确保代码能通过构建和基本功能测试
3. 更新相关文档（如 CHANGELOG.md、README.md 等）
4. 提交 PR，简要描述改动内容和测试方式

---

## 报告问题

提交 Issue 时，请尽量包含：
- 操作系统及版本
- 复现步骤
- 预期行为 vs 实际行为
- 相关日志或截图

---

> ⚠️ **免责声明**：本工具仅供学习研究使用，不构成投资建议。
