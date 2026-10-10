# 本机原生启动 WeKnora 前端（Windows 版，等价于 scripts/dev.sh frontend）
# Vite 开发服务器: http://localhost:5173，API 代理到后端 http://localhost:8080
$ErrorActionPreference = 'Stop'
$repo = $PSScriptRoot
Set-Location (Join-Path $repo 'frontend')

# 新版 pdfjs 要求 Node >= 22.13；优先复用已有的本机运行时。
$nodeVersion = if (Get-Command node -ErrorAction SilentlyContinue) {
    [version]((& node --version).TrimStart('v'))
} else { [version]'0.0' }
if ($nodeVersion -lt [version]'22.13') {
    $candidates = @()
    if ($env:TEKNOWRA_NODE_HOME) { $candidates += $env:TEKNOWRA_NODE_HOME }
    $candidates += Join-Path $env:USERPROFILE '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin'
    foreach ($candidate in $candidates) {
        $nodeExe = Join-Path $candidate 'node.exe'
        if (-not (Test-Path -LiteralPath $nodeExe)) { continue }
        $candidateVersion = [version]((& $nodeExe --version).TrimStart('v'))
        if ($candidateVersion -ge [version]'22.13') {
            $env:PATH = "$candidate;" + $env:PATH
            $nodeVersion = $candidateVersion
            break
        }
    }
}
if ($nodeVersion -lt [version]'22.13') {
    throw '请使用 Node 22.13+（推荐 Node 24），或设置 TEKNOWRA_NODE_HOME 为已有 Node 所在目录。'
}
Write-Host ">> Node $nodeVersion" -ForegroundColor Cyan

if (-not (Test-Path 'node_modules')) {
    Write-Host ">> node_modules 不存在，先装依赖..." -ForegroundColor Yellow
    npm ci --no-audit --no-fund
}

Write-Host ">> Vite dev server → http://localhost:5173" -ForegroundColor Green
npm run dev
