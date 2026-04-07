# Changelog

## [v1.1.0] - 2026-04-07

### 新增 (Features)
- **ML 双引擎预测集成**
  - **Engine A (Sentiment+Price Fusion)**: 基于 Cross-Attention 的 TextCNN+Price Encoder 模型，预测次日走势方向（上涨/持平/下跌）及异动概率。
  - **Engine B (Financial LSTM)**: 基于 BiLSTM+Self-Attention 的财务序列模型，预测 ROE、营收、M-Score 趋势方向及综合财务健康分。
  - 统一 Python ONNX 推理入口 (`ml_models/inference.py`)，Go 侧通过 `analyzer/ml_inference.go` 子进程调用并解析 JSON 结果。
  - 新增 `analyzer/ml_features.go` 负责从财报和 K 线提取模型输入特征。
  - 报告模块 9 自动渲染 Engine A/B 预测表格；模型不可用时优雅回退到财务因子简易推断。
- **RIM 估值接入实时行情**
  - 模块 7 剩余收益模型现在可直接利用实时股价、总市值、PB 计算每股净资产(BPS)、EPS 及内在价值。
  -  pessimistic / baseline / optimistic 三情景输出具体内在价值(元)及相对当前股价的涨跌幅与评级。
  - 动态解读文本根据估算上行空间提示低估/高估/中性判断。
- **十五五政策匹配度 UI 升级**
  - 政策标签改为行内 flex chip 形式，带 5 级 SVG 信号强度条，颜色与透明度随匹配等级变化。
- **主操作按钮优化**
  - `下载财报` / `18步分析` 改为同一行内联布局，增加图标，减少垂直空间占用。
- **自选列表搜索高亮与自动加载**
  - 搜索命中股票后自动滚动定位并添加金色闪烁反馈。
  - 选中自选股后自动加载最新历史报告。

### 优化 (Improvements)
- **报告子章节编号对齐**: 修正模块 6~15 内部子章节编号与模块编号不一致的问题（如模块 6 内从 `5.1/5.2` 改为 `6.1/6.2` 等）。
- **章节跳转滚动位置修正**: `handleTocJump` 改用 `getBoundingClientRect` 精确计算相对滚动容器的位置，确保跳转后模块标题始终位于可视区域最上方。
- **可比公司变更检测**: 新增 `appliedComparables` 状态与橙色脉冲动画，当已生成报告的可比公司与当前配置不一致时提醒用户重新分析。

### 修复 (Fixes)
- **ONNX 输出节点解析修复**: `inference.py` 明确指定输出节点名称，避免 `softmax` 中间节点导致返回值数量不匹配的错误。
- **导入循环修复**: 使用 `analyzer.MLKlineData` 避免 `analyzer` 包直接引用 `downloader.KlineData` 造成的测试导入循环。

---

## [v1.0.2] - 2026-04-07

### 修复 (Fixes)
- **自选列表活跃度修复**: 修复 `GetWatchlistActivity` 中股票代码格式错误（`002584.SZ` 未拆分导致所有数据源返回空），自选列表现在能正常显示 25 只股票的活跃度分值。
- **CSV 解析修复**: 给网易财经 K 线 CSV 解析器增加 `LazyQuotes = true`，避免遇到不规范引号时直接崩溃。

### 优化 (Improvements)
- **自选活跃度显示**: 自选股列表的“活跃度”由星级（⭐）改为显示具体分值（0-100 整数）。
- **表头样式优化**: 未排序时“活跃度 ⇅”保持单行显示；“股票名称”表头左对齐，与下方列表项对齐。

### 新增 (Features) - 来自 v1.0.1 累积
- **交易活跃度评分**: 基于换手率、成交额、持续性、波动性、时间结构 5 维度计算 0-100 分活跃度得分，带行业基准校正和 1-5 星评级。
- **技术形态分析**: 集成 MA/MACD/RSI/Bollinger/形态识别到分析报告。
- **蓝绿属性自动推断**: 针对台湾籍高管，通过百度百科共现匹配推断政治属性。

---

## [v1.0.1] - 2026-04-06

### 修复 (Fixes)
- **构建修复**: 修复 `app.go:334` 的 `non-constant format string` 编译错误。
- **国籍识别修复**: 增加“台湾”字样兜底判定，解决台湾籍董事长被误判为中国大陆的问题。

### 构建 (Build)
- macOS Release: `build/bin/stock-analyzer_20260406_220821.zip`
- Windows Release: `build/bin/stock-analyzer_windows_20260406_220821.zip`

---

## 2026-04-06 18:12
- 前置版本构建（上一次发布基线）。
