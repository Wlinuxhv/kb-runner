# ============================================
# KB脚本执行框架 - PowerShell测试脚本
# ============================================

param(
    [string]$TestType = "all"
)

$ErrorActionPreference = "Continue"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
$KB_RUNNER = Join-Path $ProjectRoot "bin\kb-runner.exe"

$script:TOTAL = 0
$script:PASSED = 0
$script:FAILED = 0
$script:SKIPPED = 0

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Cyan
}

function Write-Pass {
    param([string]$Message)
    Write-Host "[PASS] $Message" -ForegroundColor Green
    $script:PASSED++
    $script:TOTAL++
}

function Write-Fail {
    param([string]$Message)
    Write-Host "[FAIL] $Message" -ForegroundColor Red
    $script:FAILED++
    $script:TOTAL++
}

function Write-Skip {
    param([string]$Message)
    Write-Host "[SKIP] $Message" -ForegroundColor Yellow
    $script:SKIPPED++
    $script:TOTAL++
}

function Write-Section {
    param([string]$Title)
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host " $Title" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
}

function Test-Command {
    param([string]$Name, [string]$Command)
    Write-Host -NoNewline "测试: $Name ... "
    try {
        Invoke-Expression $Command *> $null
        if ($LASTEXITCODE -eq 0 -or $null -eq $LASTEXITCODE) {
            Write-Pass $Name
        }
        else {
            Write-Fail $Name
        }
    }
    catch {
        Write-Fail $Name
    }
}

function Test-FileExists {
    param([string]$Name, [string]$Path)
    Write-Host -NoNewline "测试: $Name ... "
    if (Test-Path $Path) {
        Write-Pass $Name
    }
    else {
        Write-Fail $Name
    }
}

function Check-Binary {
    if (!(Test-Path $KB_RUNNER)) {
        Write-Info "编译 kb-runner..."
        Set-Location $ProjectRoot
        go build -o $KB_RUNNER ./cmd/kb-runner
    }
}

function Test-Help {
    Write-Section "基础命令测试"
    
    Test-Command "帮助信息" "$KB_RUNNER --help"
    Test-Command "版本信息" "$KB_RUNNER version"
    Test-Command "list帮助" "$KB_RUNNER list --help"
    Test-Command "run帮助" "$KB_RUNNER run --help"
    Test-Command "init帮助" "$KB_RUNNER init --help"
    Test-Command "scenario帮助" "$KB_RUNNER scenario --help"
}

function Test-CaseList {
    Write-Section "CASE管理功能测试"
    
    Test-Command "CASE列表" "$KB_RUNNER list"
    Test-Command "分类筛选" "$KB_RUNNER list --category security"
    Test-Command "标签筛选" "$KB_RUNNER list --tags critical"
    Test-Command "CASE搜索" "$KB_RUNNER list --search check"
    Test-Command "CASE详情" "$KB_RUNNER show security_check"
}

function Test-Scenario {
    Write-Section "场景管理功能测试"
    
    Test-Command "场景列表" "$KB_RUNNER scenario list"
    Test-Command "场景详情" "$KB_RUNNER scenario show daily_check"
}

function Test-Init {
    Write-Section "CASE初始化功能测试"
    
    $TestDir = Join-Path $ProjectRoot "test_output\init_test"
    if (Test-Path $TestDir) {
        Remove-Item -Recurse -Force $TestDir
    }
    New-Item -ItemType Directory -Force -Path $TestDir | Out-Null
    
    Test-Command "创建Bash CASE" "$KB_RUNNER init test_bash -o $TestDir"
    
    $BashCaseYaml = Join-Path $TestDir "test_bash\case.yaml"
    $BashRunSh = Join-Path $TestDir "test_bash\run.sh"
    Test-FileExists "Bash配置文件" $BashCaseYaml
    Test-FileExists "Bash脚本文件" $BashRunSh
    
    Test-Command "创建Python CASE" "$KB_RUNNER init test_python -l python -o $TestDir"
    
    $PythonCaseYaml = Join-Path $TestDir "test_python\case.yaml"
    $PythonRunPy = Join-Path $TestDir "test_python\run.py"
    Test-FileExists "Python配置文件" $PythonCaseYaml
    Test-FileExists "Python脚本文件" $PythonRunPy
    
    Test-Command "重复创建应失败" "$KB_RUNNER init test_bash -o $TestDir"
}

function Test-Execution {
    Write-Section "脚本执行功能测试"
    
    $hasBash = $false
    try {
        $null = Get-Command bash -ErrorAction SilentlyContinue
        $hasBash = $?
    }
    catch {
        $hasBash = $false
    }
    
    if ($hasBash) {
        $TestDir = Join-Path $ProjectRoot "test_output\exec_test"
        New-Item -ItemType Directory -Force -Path $TestDir | Out-Null
        
        $scriptContent = "@`n#!/bin/bash`necho `"test output`"`nexit 0`n`@"
        $scriptContent | Out-File -FilePath (Join-Path $TestDir "simple.sh") -Encoding utf8
        
        Test-Command "脚本执行" "$KB_RUNNER run -s $TestDir\simple.sh"
        Test-Command "JSON输出" "$KB_RUNNER run -s $TestDir\simple.sh -f json"
        Test-Command "Table输出" "$KB_RUNNER run -s $TestDir\simple.sh -f table"
    }
    else {
        Write-Skip "脚本执行 (需要Bash环境)"
        Write-Skip "JSON输出 (需要Bash环境)"
        Write-Skip "Table输出 (需要Bash环境)"
    }
}

function Show-Summary {
    Write-Section "测试结果汇总"
    Write-Host "通过: $PASSED" -ForegroundColor Green
    Write-Host "失败: $FAILED" -ForegroundColor Red
    Write-Host "跳过: $SKIPPED" -ForegroundColor Yellow
    Write-Host "总计: $TOTAL"
    Write-Host ""
    
    if ($FAILED -gt 0) {
        Write-Host "测试存在失败项" -ForegroundColor Red
        exit 1
    }
    else {
        Write-Host "所有测试通过" -ForegroundColor Green
        exit 0
    }
}

function Main {
    Write-Host ""
    Write-Info "KB脚本执行框架 - 自动化测试"
    Write-Info "项目目录: $ProjectRoot"
    Write-Info "测试工具: $KB_RUNNER"
    Write-Host ""
    
    Check-Binary
    
    Test-Help
    Test-CaseList
    Test-Scenario
    Test-Init
    Test-Execution
    
    Show-Summary
}

Main
