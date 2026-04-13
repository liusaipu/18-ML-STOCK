# Stock Analyzer

基于 **Wails v2 + Go + React + TypeScript** 的跨平台股票分析桌面应用，支持对 A 股及港股进行系统的「十八步财报分析法」评估，生成专业的深度投资分析报告。

---

## ✨ 功能特性

- **自选股票管理**：支持 A 股（沪/深/创业板/科创板/北交所）+ 港股，最多 100 只，拼音/代码/名称搜索，拖拽排序
- **财报数据获取**：
  - 从东方财富网络自动下载三大表
  - 支持同花顺 CSV/Excel 手动导入
  - 自动多源校验与历史版本归档（保留最近 3 批）
- **十八步财务分析引擎**：完整覆盖审计、资产质量、偿债、盈利、现金流、ROE、成长、分红等 18 个维度
- **Beneish M-Score**：完整模型计算财报操纵风险
- **可比公司分析**：添加 3~5 家可比公司，自动计算均值/最高/最低/排名百分位，支持多年度趋势对比
- **计算过程溯源**：核心财务指标支持逐层展开，查看原始数据、计算公式与中间步骤
- **拼音首字母搜索**：股票搜索支持拼音首字母匹配（如输入"mt"匹配"茅台"）
- **自选股活跃度排序**：基于换手率、量比、振幅等指标计算活跃度评分，支持按星级排序
- **实时行情**：对接东方财富 + 腾讯财经 fallback，展示最新价、PE/PB、换手率、K 线等
- **非财务风险监控**：自动获取股权质押率、监管问询函、大股东减持等A股特有风险信号
- **股票概念与风口追踪**：获取股票所属概念板块，结合涨跌幅判断市场风口
- **报告搜索与高亮**：右栏报告支持关键词搜索，自动高亮匹配内容并支持跳转
- **RIM参数自定义**：支持手动输入Beta、无风险利率等参数进行个性化估值
- **财务数据导出**：支持将当前或历史财务数据导出为ZIP压缩包
- **分析缓存智能检测**：自动检测数据变化，提示是否需要重新分析
- **A-Score 综合风险画像**：专为 A 股构建的 6 维风险评分（M-Score + Z-Score + 现金流偏离度 + 应收账款异常 + 毛利率波动 + 非财务爬虫信号），0-100 分连续化评估，支持 5 年历史趋势与同业对比
- **投资报告生成**：16 模块专业框架（执行摘要、基本面、横向对比、政策匹配、RIM 估值、A-Score 风险画像、技术面、ML 预测、智能选股、芒格逆向检查、巴芒清单、舆情监控、投资建议等），支持 Markdown 导出
- **可比公司增量更新**：添加可比公司并下载财报后，可一键只更新报告模块4（行业横向对比），无需重新执行完整分析
- **设置面板**：右上角设置按钮，整合主题切换、图表配置（K线时间范围、均线开关）、数据配置（财报年限、自动更新）和关于信息
- **模块复制功能**：分析报告中每个模块支持一键复制为 Markdown、纯文本或图片格式，方便分享和存档
- **K线图表重构**：使用 Apache ECharts 重构技术指标图表，四图（K线+成交量、MACD、RSI、布林带）完美对齐，支持统一缩放和拖拽

---

## 📸 截图

### 深色模式
![深色模式](./docs/screenshots/screenshot-dark.png)

### 浅色模式
![浅色模式](./docs/screenshots/screenshot-light.png)

---

## 🛠️ 依赖要求

### 一、构建依赖（开发/打包需要）

| 依赖 | 版本要求 | 用途 |
|------|---------|------|
| **Go** | >= 1.22（推荐 1.26+） | 后端逻辑 |
| **Node.js** | >= 18（推荐 20+） | 前端构建 |
| **npm** / **pnpm** | 最新版 | 前端依赖管理 |
| **Wails CLI** | >= v2.12.0 | 跨平台桌面应用框架 |

**安装 Wails CLI：**
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails version  # 验证安装
```

#### macOS 额外依赖
- **Xcode Command Line Tools**（CGO 编译必需）
  ```bash
  xcode-select --install
  ```

#### Windows 额外依赖
- **WebView2 Runtime**（Windows 10/11 通常已预装）
- **gcc 编译器**：推荐 [MSYS2](https://www.msys2.org/) 安装 `mingw-w64-x86_64-gcc`

---

### 二、运行时依赖（最终用户需要）

#### 必需 ✅
| 依赖 | 用途 | 安装命令 |
|------|------|---------|
| **Python** | 3.10+ | ML 模型推理必需 | [官网下载](https://www.python.org/downloads/) |
| **onnxruntime** | ML Engine A/B | `pip install onnxruntime` |
| **scikit-learn** | ML Engine D | `pip install scikit-learn` |
| **numpy** | 数值计算 | `pip install numpy` |

**一键安装必需依赖：**
```bash
pip install onnxruntime scikit-learn numpy
```

#### 可选 ⚡（增强功能）
| 依赖 | 用途 | 影响模块 | 安装命令 |
|------|------|---------|---------|
| **akshare** | A股数据爬虫 | RIM估值、风险爬虫、概念数据 | `pip install akshare` |

**akshare 功能说明：**
- ✅ **RIM 估值**（模块7）：自动获取 Beta、无风险利率、市场风险溢价等参数
- ✅ **非财务风险**（模块8）：股权质押率、监管问询函、大股东减持数据
- ✅ **股票概念**（模块11）：概念板块、风口追踪
- ❌ **不影响**：核心 18 步财务分析、ML 预测、可比公司分析

**完整安装（推荐）：**
```bash
pip install onnxruntime scikit-learn numpy akshare
```

---

## 🚀 构建与运行

### 1. 克隆项目
```bash
git clone https://github.com/yourusername/stock-analyzer.git
cd stock-analyzer
```

### 2. 安装前端依赖
```bash
cd frontend
npm install
cd ..
```

### 3. 开发模式
```bash
# 同时启动 Go 后端 + Vite 前端热重载
wails dev
```

开发模式下会自动打开桌面应用窗口。你也可以在浏览器中访问 `http://localhost:34115` 进行前端调试（Go 方法通过 Wails JS 绑定调用）。

### 4. 构建生产版本
```bash
# 构建当前平台的应用
wails build

# 构建 macOS Universal 二进制
wails build -platform darwin/universal

# 构建 Windows 可执行文件
wails build -platform windows/amd64
```

### macOS 运行须知

**系统要求**：
- macOS 11+ (Big Sur 或更高版本)
- **Python 3.10+**（[官网下载](https://www.python.org/downloads/macos/)）

**安装步骤**：
1. 下载 `.app` 应用程序包
2. 拖入应用程序文件夹
3. **安装 Python 依赖**（必需）：
   ```bash
   pip3 install onnxruntime scikit-learn numpy
   ```
4. **（可选）安装 akshare 获取完整功能**：
   ```bash
   pip3 install akshare
   ```

**依赖检查**：
如果启动后 ML 预测显示"数据缺失"，请运行诊断脚本：
```bash
python3 ml_models/check_env.py
```

---

### Windows 运行须知

**系统要求**：
- Windows 10/11（64位）
- **Python 3.10+**（[官网下载](https://www.python.org/downloads/windows/)）
- **WebView2 Runtime**（通常已预装，如缺失会自动提示）

**安装步骤**：
1. 解压 `stock-analyzer-windows-amd64.zip`
2. 确保 `ml_models` 文件夹与 `stock-analyzer.exe` 在同一目录
3. **安装 Python 依赖**（必需）：
   ```bash
   pip install onnxruntime scikit-learn numpy
   ```
4. **（可选）安装 akshare 获取完整功能**：
   ```bash
   pip install akshare
   ```

**目录结构要求**：
```
stock-analyzer/
├── stock-analyzer.exe    # 主程序
├── ml_models/            # ML模型文件夹（必需）
│   ├── inference.py
│   ├── engine_a_sentiment/
│   ├── engine_b_financial/
│   └── engine_d_risk/
└── data/                 # 数据目录（自动生成）
```

**依赖检查**：
如果启动后 ML 预测显示"数据缺失"，请运行诊断脚本：
```bash
python ml_models/check_env.py
```

**常见问题**：
- **"推理失败: exit status 9009"** → Python 未安装或未添加到 PATH
- **"模型文件未加载"** → 检查 `ml_models` 文件夹是否在程序同级目录
- **"akshare not installed"** → 可选功能，不影响核心分析

### 5. 打包分发（脚本）
```bash
# 同时构建 macOS + Windows 并生成 zip
./build-release.sh all

# 仅构建 macOS
./build-release.sh mac

# 仅构建 Windows
./build-release.sh windows
```

---

## 📁 项目结构

```
stock-analyzer/
├── app.go                    # Wails 主应用入口（暴露前后端绑定方法）
├── main.go                   # 程序入口
├── storage.go                # 本地文件存储（自选列表、财报数据、报告、缓存）
├── csvparser.go              # 同花顺 CSV/Excel 宽容解析器
├── build-release.sh          # 一键打包脚本
├── wails.json                # Wails 配置文件
├── go.mod                    # Go 模块依赖
├── analyzer/                 # 18 步财务分析引擎
│   ├── engine.go             # 分析主入口编排
│   ├── steps.go              # 18 步逐条计算逻辑
│   ├── data.go               # 财务数据结构与科目归一化
│   ├── types.go              # 分析结果类型定义（含 CalcTrace 溯源）
│   ├── mscore.go             # Beneish M-Score 模型
│   ├── risk_analysis.go      # A-Score 综合风险评分（6 维融合）
│   ├── ascore_module.go      # A-Score 风险画像 Markdown 模块
│   ├── evaluator.go          # 扣分规则与百分制评分
│   ├── report.go             # Markdown 报告生成
│   ├── comparable.go         # 可比公司分析
│   ├── ml_inference.go       # ONNX 双引擎 ML 预测推理
│   ├── policy.go             # 十五五政策匹配评估
│   └── rim.go / rim_data.go  # 剩余收益模型（RIM）估值
├── downloader/               # 网络数据下载器
│   ├── eastmoney.go          # 东方财富 API（财报下载、行情、基本资料）
│   ├── risk_crawler.go       # 非财务风险爬虫（股权质押/问询函/减持）
│   ├── mapping.go            # 科目映射表
│   ├── validator.go          # 多源数据校验
│   └── concept.go            # 概念与风口数据获取
├── ml_models/                # Python ML 模型与爬虫脚本
│   ├── inference.py          # ONNX 双引擎推理服务
│   ├── risk_crawler.py       # 风险数据爬虫（akshare / cninfo）
│   └── *.onnx                # 导出后的 ONNX 模型文件
├── frontend/                 # React + TypeScript + Vite 前端
│   ├── src/
│   │   ├── App.tsx           # 主应用组件
│   │   ├── App.css           # 主题样式（深色/浅色）
│   │   ├── KlineChart.tsx    # K 线图表组件
│   │   ├── stocks.ts         # 股票代码库
│   │   └── wailsjs/          # Wails 自动生成的 Go 绑定
│   ├── index.html
│   ├── package.json
│   └── vite.config.ts
├── docs/
│   └── screenshots/            # 项目截图
├── 功能列表.md               # 详细功能清单
└── README.md                 # 本文件
```

---

## 📝 数据说明

- **本地存储路径**：`~/.config/stock-analyzer/`
  - `watchlist.json`：自选列表
  - `comparables.json`：可比公司配置
  - `data/{symbol}/`：当前生效的财报 JSON
  - `data/{symbol}/history/`：历史归档（最近 3 批）
  - `reports/{symbol}/`：生成的 Markdown 报告

- **数据来源**：东方财富公开 API、腾讯财经接口、用户手动导入 CSV

---

## 🗺️ 改进计划（Roadmap）

### A-Score：A 股适配综合风险评分（前两阶段已完成）

A-Score 综合风险评分已完整落地，当前实现包含：
- **财务造假层**：M-Score（15%）+ 现金流偏离度（20%）+ 应收账款异常度（15%）+ 毛利率异常波动（10%）
- **破产/偿债层**：Altman Z-Score A 股适配版（20%）
- **A 股特有非财务层**：股权质押率、监管问询函、大股东减持（20%），通过 `akshare` / `cninfo` 爬虫自动获取

#### 已完整实现 ✅

- **Engine D 风险预警模型**：以历史 A 股舞弊/退市公司为正样本，训练 LightGBM 风险预警模型，输入为 A-Score 各维度 + 技术面 + 活跃度，实现事前概率化预警。已集成到报告模块 10.3。
- **补充增强功能（v1.3.0）**：
  - 自选股活跃度排序与星级评分
  - RIM参数自定义弹窗
  - 报告搜索与高亮跳转
  - 财务数据导出ZIP
  - 拼音首字母搜索
  - 非财务风险爬虫（股权质押/问询函/减持）
  - 股票概念与风口追踪
  - 18步计算过程溯源（CalcTrace）
  - 自选股拖拽排序
  - 分析缓存智能检测

#### 尚未实现

- **智能财报问答**：基于 LLM 的财报解读对话功能。

---

## 📄 License

[MIT](./LICENSE)
