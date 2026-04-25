<!-- AGENTS.md for stock-analyzer / StockFinLens -->

# Agent Guidelines for StockFinLens

> 本文件面向 AI 编码助手。项目主要使用**中文**进行注释和文档编写。

## 项目概述

**StockFinLens（股票财报透镜）** 是一款基于 Wails v2 的跨平台桌面股票财报透视分析工具，支持 A 股与港股。它通过多层分析引擎（18 步财务透视、A-Score 风险评分、可比公司横向对比、ML 三引擎预测、RIM 估值、技术形态与交易活跃度分析等）生成深度 Markdown 分析报告。

- **模块名**: `github.com/liusaipu/stockfinlens`
- **当前版本**: `1.3.25`（见 `wails.json` 与 `frontend/src/Settings.tsx`）
- **本地数据目录**: `~/.config/stock-analyzer/`

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面框架 | Wails v2 (Go 1.25 + WebView2) |
| 后端 | Go 1.25，模块 `github.com/liusaipu/stockfinlens` |
| 前端 | React 18 + TypeScript + Vite |
| 图表 | Apache ECharts v6、lightweight-charts v5、recharts |
| ML 推理 | Python 3 + ONNX Runtime + scikit-learn + numpy |
| 数据获取 | 东方财富 API、腾讯财经接口、akshare（港股）、同花顺 CSV/Excel 导入 |
| Excel/CSV | `github.com/xuri/excelize/v2`、标准库 `encoding/csv` |
| 通知 | `git.sr.ht/~jackmordaunt/go-toast/v2`（Windows Toast） |

## 项目结构

```
stock-analyzer/
├── main.go                  # Wails 入口，embed 前端资源、股票库、ML 模型
├── app.go                   # App 结构体，所有 Wails 绑定方法（~2500 行）
├── storage.go               # 本地文件存储管理器（JSON 持久化）
├── csvparser.go             # 同花顺 CSV/Excel 财报导入解析器
├── integration_test.go      # 集成测试
├── wails.json               # Wails 配置（版本号来源之一）
├── go.mod / go.sum          # Go 依赖
├── requirements.txt         # Python 依赖（pandas, numpy, akshare, requests, openpyxl, tqdm）
├── build-windows.ps1        # Windows 构建/打包脚本
├── build-release.sh         # macOS/Windows 发布构建脚本
│
├── analyzer/                # 核心分析引擎
│   ├── engine.go            # 18 步分析主入口（RunAnalysisWithAll 等）
│   ├── steps.go             # 18 步财务透视具体实现
│   ├── evaluator.go         # 评分与评级逻辑
│   ├── report.go            # Markdown 报告生成器（16 个模块）
│   ├── ascore_module.go     # A-Score 风险评分（M-Score、Z-Score、非财务信号）
│   ├── risk_analysis.go     # 风险雷达与亮点/风险提取
│   ├── comparable.go        # 可比公司聚焦分析
│   ├── industry.go          # 行业均值数据库与横向对比
│   ├── policy.go            # 十五五政策匹配度评估
│   ├── rim.go               # RIM 剩余收益模型估值
│   ├── technical.go         # 技术形态分析（MA/MACD/KDJ/布林带）
│   ├── activity.go          # 交易活跃度分析（换手率/量比/金额得分）
│   ├── ml_features.go       # ML 特征工程（构建 Engine A/B/D 输入）
│   ├── ml_inference.go      # 调用 Python ONNX 推理脚本
│   ├── data.go              # 财务数据加载与清洗
│   ├── types.go             # 核心类型定义（CalcTrace、StepResult、AnalysisReport 等）
│   └── *_test.go            # 单元测试
│
├── downloader/              # 数据下载与爬取层
│   ├── eastmoney.go         # 东方财富 API（财报、资料、行情、K线）
│   ├── tencent.go           # 腾讯财经备用数据源
│   ├── sentiment.go         # 舆情情绪数据抓取
│   ├── risk_crawler.go      # A-Score 非财务风险爬虫（股权质押、问询函、减持）
│   ├── rim_data.go          # RIM 外部参数获取（无风险利率、Beta、EPS 预测）
│   ├── industry_updater.go  # 行业均值数据库更新
│   ├── policy_updater.go    # 政策库更新
│   ├── concept.go           # 股票概念与风口数据
│   ├── mapping.go           # 科目名映射与标准化
│   ├── validator.go         # 多源数据校验
│   └── *_test.go            # 单元测试与集成测试
│
├── frontend/                # React 前端
│   ├── package.json         # 前端依赖（echarts, react, vite 等）
│   ├── src/
│   │   ├── App.tsx          # 主界面组件（~2700 行，自选股、分析、报告展示）
│   │   ├── Settings.tsx     # 设置面板（版本号来源之二）
│   │   ├── UnifiedChart.tsx # K线统一图表组件
│   │   ├── KlineChart.tsx   # K线图表
│   │   ├── FinancialTrendDrawer.tsx # 财务趋势抽屉
│   │   └── stocks.ts        # 内置股票代码库
│   └── wailsjs/             # Wails 生成的 Go 绑定代码
│
├── ml_models/               # ML 模型与推理脚本（打包时必须包含）
│   ├── inference.py         # 统一推理入口（Engine A/B/D）
│   ├── engine_a_sentiment/  # 情绪+量价融合 ONNX 模型
│   ├── engine_b_financial/  # 财务 LSTM ONNX 模型 + scaler.pkl
│   └── engine_d_risk/       # LightGBM 风险预警模型
│
├── scripts/                 # Python 数据脚本（打包时必须包含）
│   ├── fetch_all_industry_data.py   # 全市场行业数据采集
│   ├── fetch_hk_financials.py       # 港股财报获取
│   ├── fetch_hk_profile.py          # 港股基本资料获取
│   ├── fetch_rim_data.py            # RIM 外部数据获取
│   └── ...
│
├── data/
│   └── stocks.json          # 内置 A 股/港股代码库（被 embed）
│
└── docs/                    # 文档与截图
```

## 构建与运行

### 环境要求

- **Go** >= 1.25
- **Node.js** >= 18
- **Python** 3.10+（运行时必需，用于 ML 推理与数据脚本）
- **Wails CLI** >= v2.12：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **GCC / MinGW-w64**（Windows 构建必需）

### 安装依赖

```bash
# Go 依赖
go mod tidy

# 前端依赖
cd frontend && npm install

# Python 依赖（运行时必需）
pip install -r requirements.txt
# 核心运行时依赖：onnxruntime, scikit-learn, numpy, pandas
```

### 常用命令

```bash
# 开发模式（热重载前端 + Go）
wails dev

# 构建生产版本（Windows）
# 推荐使用脚本，因为它会自动执行版本一致性校验：
.\build-windows.ps1 build

# 打包为 ZIP（包含 ml_models 和 scripts）
.\build-windows.ps1 package

# 运行 Go 测试
go test ./...

# 运行特定包测试
go test ./analyzer/...
go test ./downloader/...
```

### 构建注意事项

1. **版本号一致性（硬要求）**: `wails.json` 中的 `info.productVersion` 必须与 `frontend/src/Settings.tsx` 中的 `const version` 完全一致。两个构建脚本都会校验此项，不一致会**中断构建**。
2. **前端 dist 重建**: 如果前端代码有变更，构建前必须确保 `frontend/dist` 是最新的。Wails `build` 在 `dist` 已存在时可能跳过前端构建，导致打包旧代码。
3. **打包产物必须包含**: `ml_models/` 和 `scripts/` 目录。Go 后端在运行时会从可执行文件同级目录查找这些路径。
4. **开发模式 vs 生产模式**: `main.go` 中 `readStockJSON()` 优先读取本地 `data/stocks.json`，打包后 fallback 到 `embed.FS`。

## 代码组织规范

### 包划分

| 包 | 职责 |
|----|------|
| `main` | Wails 应用生命周期、App 绑定方法、存储管理、CSV 解析 |
| `analyzer` | 纯分析逻辑，不依赖网络，输入为本地财务数据 + 外部传入的行情/舆情/可比公司数据 |
| `downloader` | 所有网络 I/O：财报下载、行情、K线、舆情、风险爬虫、外部数据获取 |

### 核心数据流

1. 用户在 `App.tsx` 选择股票 -> 调用 `AddToWatchlist`
2. 下载财报：`DownloadReports` -> `downloader.DownloadFinancialReports` -> 保存到 `~/.config/stock-analyzer/data/{symbol}/`
3. 执行分析：`AnalyzeStock` -> `analyzer.RunAnalysisWithAll` -> 生成 `AnalysisReport` -> 保存 Markdown 报告与 JSON 快照
4. 前端读取快照恢复亮点/风险面板，读取 Markdown 渲染报告

### 股票代码格式

- A 股上海：`603501.SH`
- A 股深圳：`000001.SZ`
- 港股：`00700.HK`
- 内部存储和 UI 传递均使用上述带点格式

### 缓存策略

| 数据类型 | 缓存位置 | 有效期 |
|----------|----------|--------|
| 实时行情 | `data/{symbol}/quote.json` | 15 分钟 |
| 舆情情绪 | `data/{symbol}/sentiment.json` | 60 分钟 |
| 股票资料 | `data/{symbol}/profile.json` | 7 天 |
| 活跃度 | `data/{symbol}/activity.json` | 1 天 |
| RIM 外部数据 | `data/{symbol}/rim_cache.json` | 12 小时 |
| K线数据 | `data/{symbol}/klines.json` | 持久（分析时写入） |
| 分析报告 | `reports/{symbol}/latest.md` | 每次分析覆盖 |
| 分析快照 | `snapshots/{symbol}.json` | 每次分析覆盖 |

### 分析引擎的 18 步流程

在 `analyzer/engine.go` 中按顺序执行：
1. 审计意见 → 2. 资产规模 → 3. 偿债能力 → 4. 竞争地位 → 5. 应收账款 → 6. 固定资产 → 7. 投资资产 → 8. **风险分析（A-Score）** → 9. 营收增长 → 10. 毛利率 → 11. 运营效率 → 12. 成本控制 → 13. 费用率 → 14. 核心利润 → 15. 现金流质量 → 16. ROE → 17. 资本支出 → 18. 分红政策

## 测试策略

```bash
# 全部测试
go test ./...

# 仅 analyzer 包测试
go test ./analyzer/...

# 仅 downloader 包测试
go test ./downloader/...

# 集成测试（根目录）
go test -run TestIntegration
```

测试文件分布：
- `analyzer/activity_test.go` — 活跃度计算测试
- `analyzer/ascore_validation_test.go` — A-Score 验证测试
- `analyzer/policy_test.go` — 政策匹配测试
- `analyzer/report_test.go` — 报告生成测试
- `downloader/analyzer_integration_test.go` — 下载器与分析器集成测试
- `downloader/downloader_test.go` — 下载器单元测试
- `storage_test.go` — 存储层测试
- `integration_test.go` — 端到端集成测试

## 发布流程

1. **版本号同步**: 确保 `wails.json` 与 `frontend/src/Settings.tsx` 版本一致。
2. **更新 CHANGELOG.md**: 在顶部追加新版本说明。
3. **提交并推送**: `git commit`, `git push origin main`
4. **打标签**: `git tag v1.3.25`, `git push origin v1.3.25`
5. **构建发布包**:
   - Windows: `.\build-windows.ps1 package`
   - macOS: `./build-release.sh mac`
6. **创建 GitHub Release**: 上传 ZIP，将 `CHANGELOG.md` 对应章节粘贴到 Release Notes。
7. **版本号递增**: 发布后立刻将两个文件版本号 bump 到下一个未发布版本（如 `1.3.26`）。

## 修改注意事项

- **不要破坏 Wails 绑定**: `app.go` 中导出的方法（首字母大写）会被前端调用，修改签名必须同步更新 `frontend/src/App.tsx` 中的调用。
- **Python 脚本路径解析**: `ml_models/inference.py` 和 `scripts/*.py` 在开发模式与打包后的路径解析逻辑不同。开发时通过 `runtime.Caller(0)` 向上查找项目根目录；打包后优先使用 `os.Executable()` 所在目录。新增 Python 脚本时请遵循同样模式。
- **Windows 隐藏窗口**: 所有通过 `exec.Command` 调用 Python 脚本的地方，在 Windows 上必须设置 `syscall.SysProcAttr{HideWindow: true}`，否则会出现 CMD 黑框。
- **新增 Go 依赖**: 本项目不使用 `vendor`，直接通过 `go mod` 管理。新增依赖后运行 `go mod tidy`。
- **前端状态管理**: `frontend/src/App.tsx` 是单文件大组件（~2700 行），新增功能时优先在现有 hooks 体系内扩展，避免引入额外状态管理库。
- **中文注释**: 所有代码注释和文档保持中文。提交信息也建议使用中文。
