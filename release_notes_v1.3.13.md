## [v1.3.13] - 2026-04-14

### 修复 (Fixes)
- **Settings 数据管理提示状态解耦**
  - 将 `settingsActionStatus` 拆分为 `policyActionStatus` 与 `industryActionStatus`，更新政策库/行业数据库的成功或失败提示仅显示在对应按钮下方，不再互相串位
- **行业数据库更新提示时长调整**
  - 更新成功后的轻量提示条保留时间从 3 秒延长至 5 秒，与失败提示保持一致
- **彻底移除 Settings 中的 alert 弹窗**
  - 政策库与行业数据库更新的反馈全部改用按钮下方轻量提示条，不再调用 `alert()`

### 优化 (Improvements)
- **模块 4.2 信息弹窗交互优化**
  - 可比公司明细标题右侧的 ℹ️ 说明弹窗，现支持点击弹窗外部任意区域自动关闭，无需再次点击图标

### 平台安装注意
- **Windows**：需要预装 Python 3.10+，并执行 `pip install onnxruntime scikit-learn numpy akshare`；需安装 WebView2 Runtime（Win10/11 通常已预装）
- **macOS**：需要预装 Python 3.10+，并执行 `pip3 install onnxruntime scikit-learn numpy akshare`；首次运行若提示安全拦截，需在"系统设置 > 隐私与安全性"中允许
