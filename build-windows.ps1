# build-windows.ps1 - Windows 构建脚本
# 用法: .\build-windows.ps1 [dev|build|package|clean]

param(
    [Parameter(Position=0)]
    [ValidateSet("dev", "build", "package", "clean", "setup")]
    [string]$Command = "build"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = $PSScriptRoot
$BinDir = "$ProjectRoot\build\bin"
$FrontendDir = "$ProjectRoot\frontend"

function Write-Info($msg) { Write-Host "[INFO] $msg" -ForegroundColor Cyan }
function Write-Success($msg) { Write-Host "[OK] $msg" -ForegroundColor Green }
function Write-Error($msg) { Write-Host "[ERROR] $msg" -ForegroundColor Red }

function Test-Command($cmd) {
    return [bool](Get-Command -Name $cmd -ErrorAction SilentlyContinue)
}

function Setup-Environment {
    Write-Info "检查开发环境..."
    
    # 检查 Go
    if (-not (Test-Command "go")) {
        Write-Error "Go 未安装，请从 https://go.dev/dl/ 下载安装"
        exit 1
    }
    $goVersion = go version
    Write-Success "Go 版本: $goVersion"
    
    # 检查 Node.js
    if (-not (Test-Command "node")) {
        Write-Error "Node.js 未安装，请从 https://nodejs.org/ 下载安装"
        exit 1
    }
    $nodeVersion = node --version
    Write-Success "Node.js 版本: $nodeVersion"
    
    # 检查 npm
    if (-not (Test-Command "npm")) {
        Write-Error "npm 未安装"
        exit 1
    }
    Write-Success "npm 版本: $(npm --version)"
    
    # 检查 Wails
    if (-not (Test-Command "wails")) {
        Write-Info "安装 Wails CLI..."
        go install github.com/wailsapp/wails/v2/cmd/wails@latest
        if (-not $?) {
            Write-Error "Wails 安装失败"
            exit 1
        }
    }
    Write-Success "Wails: $(wails version)"
    
    # 检查 GCC (MinGW)
    if (-not (Test-Command "gcc")) {
        Write-Warning "GCC 未找到。请安装 MinGW-w64:"
        Write-Host "  方式1: choco install mingw"
        Write-Host "  方式2: 从 https://www.msys2.org/ 安装"
        exit 1
    }
    Write-Success "GCC: $(gcc --version | Select-Object -First 1)"
    
    # 检查 Python
    $pythonCmd = if (Test-Command "python") { "python" } elseif (Test-Command "python3") { "python3" } else { $null }
    if (-not $pythonCmd) {
        Write-Warning "Python 未安装，某些功能可能不可用"
    } else {
        Write-Success "Python: $(& $pythonCmd --version)"
    }
    
    Write-Success "环境检查完成"
}

function Install-FrontendDeps {
    Write-Info "安装前端依赖..."
    Set-Location $FrontendDir
    
    if (-not (Test-Path "node_modules")) {
        npm install
        if (-not $?) {
            Write-Error "npm install 失败"
            exit 1
        }
    } else {
        Write-Info "node_modules 已存在，跳过安装"
    }
    
    Set-Location $ProjectRoot
    Write-Success "前端依赖安装完成"
}

function Setup-PythonVenv {
    Write-Info "设置 Python 虚拟环境..."
    
    if (Test-Path ".venv") {
        Write-Info "删除 macOS 虚拟环境..."
        Remove-Item -Recurse -Force ".venv"
    }
    
    $pythonCmd = if (Test-Command "python") { "python" } else { "python3" }
    
    Write-Info "创建 Windows 虚拟环境..."
    & $pythonCmd -m venv .venv
    if (-not $?) {
        Write-Error "虚拟环境创建失败"
        exit 1
    }
    
    Write-Info "安装 Python 依赖..."
    .\.venv\Scripts\pip.exe install -r requirements.txt
    if (-not $?) {
        Write-Warning "pip install 失败，可能需要手动安装依赖"
    }
    
    Write-Success "Python 环境设置完成"
}

function Start-Dev {
    Write-Info "启动开发模式..."
    Install-FrontendDeps
    wails dev
}

function Build-Project {
    param([string]$Platform = "windows/amd64")
    
    Write-Info "构建 Windows 版本 ($Platform)..."
    Install-FrontendDeps
    
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    
    $env:CGO_ENABLED = "1"
    wails build -platform $Platform -clean
    
    if (-not $?) {
        Write-Error "构建失败"
        exit 1
    }

    # 复制 ml_models 和 scripts 到构建目录
    Write-Info "复制 ml_models 和 scripts 到构建目录..."
    Copy-Item -Recurse -Force "$ProjectRoot\ml_models" "$BinDir\"
    Copy-Item -Recurse -Force "$ProjectRoot\scripts" "$BinDir\"
    
    Write-Success "构建完成: $BinDir\stock-analyzer.exe"
}

function Package-Build {
    Write-Info "打包 Windows 版本..."
    
    $wailsJson = Get-Content "$ProjectRoot\wails.json" -Raw | ConvertFrom-Json
    $version = $wailsJson.info.productVersion
    $zipName = "stock-analyzer-windows-amd64-v${version}.zip"
    $zipPath = "$BinDir\$zipName"
    
    Build-Project
    
    Write-Info "创建压缩包: $zipName"
    Compress-Archive -Path "$BinDir\stock-analyzer.exe","$BinDir\ml_models","$BinDir\scripts" -DestinationPath $zipPath -Force
    
    $size = (Get-Item $zipPath).Length / 1MB
    Write-Success "打包完成: $zipPath ($([math]::Round($size, 2)) MB)"
}

function Clean-Project {
    Write-Info "清理构建文件..."
    
    $paths = @(
        "build",
        "frontend\dist",
        "frontend\node_modules",
        ".venv"
    )
    
    foreach ($path in $paths) {
        $fullPath = Join-Path $ProjectRoot $path
        if (Test-Path $fullPath) {
            Write-Info "删除: $path"
            Remove-Item -Recurse -Force $fullPath
        }
    }
    
    Write-Success "清理完成"
}

# 主逻辑
Write-Host "================================" -ForegroundColor Blue
Write-Host "  Stock Analyzer Windows Build" -ForegroundColor Blue
Write-Host "================================" -ForegroundColor Blue

switch ($Command) {
    "setup" {
        Setup-Environment
        Install-FrontendDeps
        Setup-PythonVenv
    }
    "dev" {
        Setup-Environment
        Start-Dev
    }
    "build" {
        Setup-Environment
        Build-Project
    }
    "package" {
        Setup-Environment
        Package-Build
    }
    "clean" {
        Clean-Project
    }
}

Write-Host "================================" -ForegroundColor Blue
