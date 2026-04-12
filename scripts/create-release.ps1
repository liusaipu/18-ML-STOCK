#!/usr/bin/env pwsh
# GitHub Release 创建脚本
# 用法: .\scripts\create-release.ps1 -Tag "v1.3.5"

param(
    [Parameter(Mandatory=$true)]
    [string]$Tag
)

$ErrorActionPreference = "Stop"

# 检查 gh CLI
if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    Write-Error "需要安装 GitHub CLI (gh)。安装命令: winget install GitHub.cli"
    exit 1
}

# 检查登录状态
$authStatus = gh auth status 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error "请先登录 GitHub: gh auth login"
    exit 1
}

$Repo = "liusaipu/18-ML-STOCK"
$BinDir = "build\bin"
$ZipFile = "stock-analyzer_windows_amd64_v1.3.5.zip"
$NotesFile = "RELEASE_NOTES_v1.3.5.md"

# 检查文件存在
if (-not (Test-Path "$BinDir\$ZipFile")) {
    Write-Error "找不到发布包: $BinDir\$ZipFile"
    Write-Host "请先运行构建: wails build -platform windows/amd64"
    exit 1
}

# 创建 Release
Write-Host "🚀 创建 GitHub Release $Tag..." -ForegroundColor Cyan

$ReleaseNotes = @"
Stock Analyzer $Tag 发布

## 快速开始
1. 下载 stock-analyzer_windows_amd64_v1.3.5.zip
2. 解压到任意目录
3. 安装 Python 依赖: pip install onnxruntime scikit-learn numpy
4. 运行 stock-analyzer.exe

## 详细说明
详见 RELEASE_NOTES_v1.3.5.md

## 系统要求
- Windows 10/11 (64位)
- Python 3.10+
"@

# 创建 release
git tag $Tag 2>$null
git push origin $Tag 2>$null

gh release create $Tag `
    --repo $Repo `
    --title "Stock Analyzer $Tag" `
    --notes $ReleaseNotes `
    "$BinDir\$ZipFile#Windows 版本 (v1.3.5)" `
    "$BinDir\$NotesFile#发布说明"

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Release 创建成功!" -ForegroundColor Green
    Write-Host "🔗 访问: https://github.com/$Repo/releases/tag/$Tag"
} else {
    Write-Error "Release 创建失败"
}
