# Stock Analyzer 手机版开发准备指南

> 本文档记录 iOS App 开发所需的技术选型、环境准备、发布流程及注意事项。

---

## 一、推荐语言组合

### 最佳组合（推荐）

| 层级 | 语言/框架 | 理由 |
|------|----------|------|
| **iOS App** | **Swift + SwiftUI** | 苹果官方主推，性能最优，UI 开发效率最高，完美适配 iOS 17+ 新特性（小组件、灵动岛） |
| **服务端 API** | **Go + Gin/Fiber** | 复用现有 18 步分析引擎，编译快、并发强、部署简单 |
| **数据库** | **PostgreSQL** | 关系型数据（用户、自选股、历史报告），稳定成熟 |
| **缓存** | **Redis** | 热点行情、健康分缓存、推送队列 |
| **消息推送** | **APNs (Apple Push Notification service)** | iOS 原生推送，服务端用 Go 的 `sideshow/apns2` 库 |

### 备选方案

| 场景 | 备选组合 | 适用情况 |
|------|---------|---------|
| **同时要做 Android** | **Flutter (Dart) + Go 后端** | 一套 UI 代码跑双端，适合预算有限的小团队 |
| **想尽快验证 MVP** | **React Native + Go 后端** | 找前端工程师容易，但 iOS 体验不如 SwiftUI |
| **服务端不想自己维护** | **SwiftUI + 云服务（阿里云/腾讯云 Serverless + Go 函数）** | 按调用付费，无服务器运维成本 |

---

## 二、iOS App 开发前的准备

### 1. 硬件要求
- **Mac 电脑**：必须有一台 Mac（Mac mini、MacBook Air/Pro 均可），iOS 开发只能在 macOS 上进行
- **iPhone 真机**：虽然 Xcode 有模拟器，但推送、相机、灵动岛、性能测试必须在真机上验证
- **稳定的网络**：下载 Xcode 约 10GB+

### 2. 软件安装
```bash
# 1. 安装 Xcode（App Store 或 Apple Developer 网站）
# 2. 安装 Xcode Command Line Tools
xcode-select --install

# 3. 安装 Homebrew（包管理器）
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 4. 可选：安装 CocoaPods（如果用第三方库）
sudo gem install cocoapods
```

### 3. 注册 Apple Developer 账号

| 类型 | 费用 | 用途 |
|------|------|------|
| **个人开发者** | **$99/年（约 ¥688）** | 个人或小团队，App Store 上架 |
| **公司开发者** | **$99/年** | 以公司名义发布，需 D-U-N-S 企业编号 |
| **企业开发者** | **$299/年** | 内部员工分发，不经过 App Store |

**建议**：先用 **个人开发者账号** 启动，等产品成熟后再转公司账号。

### 4. 申请 D-U-N-S 编号（如果走公司账号）
- 免费申请，但审核周期 **2-4 周**
- 需要营业执照、公章等
- 建议提前申请，不要等开发完了再办

---

## 三、iOS 开发注意事项

### 1. 系统权限与隐私声明
iOS 对用户隐私要求极严，必须在 `Info.plist` 中声明：

```xml
<!-- 如果需要推送 -->
<key>UIBackgroundModes</key>
<array>
    <string>fetch</string>
    <string>remote-notification</string>
</array>

<!-- 如果使用网络 -->
<key>NSAppTransportSecurity</key>
<dict>
    <key>NSAllowsArbitraryLoads</key>
    <false/>
</dict>
```

### 2. 后台运行限制
- iOS **不允许** App 在后台持续运行
- 你的批量计算必须在 **服务端** 完成，iOS 端只能接收推送
- 后台刷新（Background Fetch）每周只有几次机会，不能依赖

### 3. 网络请求安全
- 默认强制 HTTPS（ATS）
- 如果服务端暂时用 HTTP，需要在 `Info.plist` 中配置例外域名
- **建议**：服务端直接配置 SSL 证书，用 HTTPS

### 4. 数据存储策略

| 数据类型 | 存储方式 |
|---------|---------|
| 用户登录态 / 设置 | `UserDefaults` 或 `Keychain` |
| 自选股列表 / 缓存报告 | `Core Data` 或 `SwiftData` |
| 大文件（下载的财报 PDF） | 应用沙盒 `Documents` 目录 |
| 云端同步数据 | 服务端 PostgreSQL + iCloud 备选 |

### 5. 热更新限制
- iOS **禁止**动态下发可执行代码（如 React Native 的热更新、Flutter 的 Code Push）
- 纯业务逻辑更新需要走 App Store 审核
- **workaround**：把业务规则配置化，从服务端拉取配置（如卡片排序规则、阈值参数）

---

## 四、App Store 上架准备

### 上架前材料清单

| 材料 | 要求 | 建议 |
|------|------|------|
| **App 图标** | 1024×1024 PNG | 设计简洁、辨识度高的图标 |
| **截图** | iPhone 6.7"、6.5"、5.5" 三种尺寸各 3-5 张 | 用真实数据截图，不要纯 UI 占位 |
| **宣传文本** | 简短描述（80 字符内）+ 详细描述（4000 字符内） | 突出「3 分钟体检」、「A-Score」等核心卖点 |
| **隐私政策网址** | 必须 | 可用 GitHub Pages 或语雀快速生成 |
| **联系邮箱 / 电话** | 必须 | 用于苹果审核沟通 |

### 审核常见被拒原因

1. **功能不完整 / 崩溃** — 确保 MVP 版本稳定，不要有明显 Bug
2. **缺少隐私政策** — 涉及股票、财务数据，必须提供隐私政策
3. **使用非公开 API** — 只调用苹果公开文档中的 API
4. **误导性描述** — 不要写「保证收益」「稳赚不赔」等违规宣传语
5. **登录功能强制要求** — 如果核心功能需要登录，审核时需要提供测试账号
6. **金融类 App 特殊要求** — 股票分析类 App 可能被要求补充说明「不提供具体投资建议」

### 审核加速技巧
- **TestFlight 内测**：先邀请 20-50 个用户内测，收集反馈后再正式提交
- **预审沟通**：如果功能复杂，可在 App Store Connect 里备注说明
- **首次提交避免大更新**：第一次上架尽量功能精简、稳定，后续迭代再丰富

---

## 五、发布时间线参考

```
第 1 周    注册 Apple Developer 账号（个人当天通过，公司需 2-4 周）
第 2-3 周  开发 MVP + TestFlight 内测
第 4 周    准备上架材料，提交 App Store 审核
第 5 周    审核反馈修改（如果被拒）
第 6 周    正式上线，开始推广运营
```

---

## 六、总结建议

| 决策项 | 推荐选择 |
|--------|---------|
| **iOS 开发语言** | **Swift + SwiftUI** |
| **服务端** | **Go + Gin** |
| **数据库** | **PostgreSQL + Redis** |
| **开发者账号** | **个人账号（$99/年）先启动** |
| **真机测试** | 至少 1 台 iPhone（建议 iPhone 14/15 系列） |
| **内测分发** | **TestFlight** |
| **上架策略** | 精简功能首版上架，快速迭代 |

---

## 七、启动前立即执行清单

- [ ] 注册 Apple Developer 个人账号（$99/年）
- [ ] 准备 Mac 电脑 + iPhone 真机
- [ ] 安装 Xcode 和 Command Line Tools
- [ ] 确定服务端部署方案（自有服务器 / 云厂商）
- [ ] 注册域名并配置 SSL 证书（HTTPS 必需）
- [ ] 准备隐私政策页面
