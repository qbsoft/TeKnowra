# 本机原生启动 WeKnora 后端（Windows 版，等价于 scripts/dev.sh app）
#
# 与 dev.sh 的差别：dev.sh 把 REDIS_ADDR / DB_HOST 等硬编码成 localhost:6379、
# localhost:9000 等默认端口；本机已有 PostgreSQL(5432) 和 Redis(6379) 在跑，
# 容器版改用 15432 / 16379，所以这里一律以 .env 里的值为准，不做覆盖。
$ErrorActionPreference = 'Stop'
$repo = $PSScriptRoot
Set-Location $repo

# ---- Go 工具链 + MinGW-w64（均为绿色版，不写系统 PATH）----
# CGO 是必需的：依赖 github.com/duckdb/duckdb-go-bindings（预编译静态库，
# 链接参数含 -lstdc++ --static）和 github.com/asg017/sqlite-vec-go-bindings/cgo。
# CGO_ENABLED=0 时这两个包的 Go 文件会被 build constraint 全部排除，直接编译失败。
$env:GOROOT = 'D:\go'
$env:GOPATH = 'D:\gopath'
$env:PATH = "$env:GOROOT\bin;$env:GOPATH\bin;D:\mingw64\bin;" + $env:PATH
$env:CGO_ENABLED = '1'
# sqlite-vec 的 C 源码 #include "sqlite3.h"，官方 Docker builder 靠 apt 的
# libsqlite3-dev 提供该头文件；Windows 上没有，这里指向从 mattn/go-sqlite3
# 模块里取出的同版本头文件（SQLite 3.46.1），保证与最终链入二进制的
# amalgamation 版本一致。
$env:CGO_CFLAGS = '-ID:/sqlite-include -Wno-deprecated-declarations'

# ---- 加载 .env，再用 .env.local 覆盖 ----
function Import-DotEnv([string]$path) {
    if (-not (Test-Path $path)) { return }
    # 必须显式按 UTF-8 读。PowerShell 5.1 的 Get-Content 对无 BOM 的文件按系统
    # ANSI 代码页（本机为 GBK）解码，中文注释里的某些字节会被当成 GBK 双字节字符的
    # 前导字节而吞掉紧随其后的换行符，把两行粘成一行；被粘上去的那行以 # 开头，
    # 于是整行配置被当注释静默跳过。实测 717 行的 .env 用 Get-Content 只读出 697 行。
    foreach ($line in [System.IO.File]::ReadAllLines($path, [System.Text.UTF8Encoding]::new($false))) {
        $t = $line.Trim()
        if ($t -eq '' -or $t.StartsWith('#')) { continue }
        $i = $t.IndexOf('=')
        if ($i -lt 1) { continue }
        $k = $t.Substring(0, $i).Trim()
        $v = $t.Substring($i + 1).Trim()
        if ($v.Length -ge 2) {
            $q = $v[0]
            if (($q -eq '"' -or $q -eq "'") -and $v[$v.Length - 1] -eq $q) {
                $v = $v.Substring(1, $v.Length - 2)
            }
        }
        Set-Item -Path "env:$k" -Value $v
    }
}
Import-DotEnv (Join-Path $repo '.env')
Import-DotEnv (Join-Path $repo '.env.local')

# ---- 本地文件存储目录：.env.example 的 /data/files 是容器内路径，本机跑要换 ----
if (-not $env:LOCAL_STORAGE_BASE_DIR -or $env:LOCAL_STORAGE_BASE_DIR -eq '/data/files') {
    $env:LOCAL_STORAGE_BASE_DIR = Join-Path $repo '.local-data\files'
}
New-Item -ItemType Directory -Force -Path $env:LOCAL_STORAGE_BASE_DIR | Out-Null

Write-Host "DB      : $($env:DB_HOST):$($env:DB_PORT)/$($env:DB_NAME)" -ForegroundColor Cyan
Write-Host "Redis   : $($env:REDIS_ADDR)" -ForegroundColor Cyan
Write-Host "Docread : $($env:DOCREADER_ADDR)" -ForegroundColor Cyan
Write-Host "MinIO   : $($env:MINIO_ENDPOINT)" -ForegroundColor Cyan
Write-Host "Neo4j   : $($env:NEO4J_URI) (enable=$($env:NEO4J_ENABLE))" -ForegroundColor Cyan
Write-Host ""

# protoregistry.conflictPolicy=warn：与 Makefile/.air.toml 保持一致，
# 否则重复注册的 proto 会在启动时直接 panic。
$ldflags = "-X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'"

# 装了 Air 就用热重载，否则普通模式
$air = Get-Command air -ErrorAction SilentlyContinue
if ($air) {
    Write-Host ">> 检测到 Air，热重载模式启动" -ForegroundColor Green
    & air
} else {
    Write-Host ">> 编译中（首次约几分钟，之后走 build cache 很快）..." -ForegroundColor Yellow
    & go build "-ldflags=$ldflags" -o .\tmp\WeKnora.exe .\cmd\server
    if ($LASTEXITCODE -ne 0) { Write-Host "编译失败" -ForegroundColor Red; exit 1 }
    Write-Host ">> 启动后端 → http://localhost:8080" -ForegroundColor Green
    # PowerShell 5.1 会把原生程序写到 stderr 的每一行包成 ErrorRecord；
    # 后端的正常日志就走 stderr，配合 $ErrorActionPreference='Stop' 会让
    # 第一行日志直接终止脚本。这里必须放开。
    $ErrorActionPreference = 'Continue'
    & .\tmp\WeKnora.exe
}
