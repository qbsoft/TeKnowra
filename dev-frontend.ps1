# 本机原生启动 WeKnora 前端（Windows 版，等价于 scripts/dev.sh frontend）
# Vite 开发服务器: http://localhost:5173，API 代理到后端 http://localhost:8080
$ErrorActionPreference = 'Stop'
$repo = $PSScriptRoot
Set-Location (Join-Path $repo 'frontend')

if (-not (Test-Path 'node_modules')) {
    Write-Host ">> node_modules 不存在，先装依赖..." -ForegroundColor Yellow
    npm ci --no-audit --no-fund
}

Write-Host ">> Vite dev server → http://localhost:5173" -ForegroundColor Green
npm run dev
