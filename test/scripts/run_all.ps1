# ============================================
# KB Runner - Test Runner
# Run all test scripts
# ============================================

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
Write-Host " KB Runner - Test Runner" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta
Write-Host ""

$totalPassed = 0
$totalFailed = 0

function Run-TestScript {
    param($scriptPath, $scriptName)
    Write-Host ""
    Write-Host ">>> Running: $scriptName" -ForegroundColor Yellow
    
    $output = & powershell -ExecutionPolicy Bypass -File $scriptPath 2>&1
    $exitCode = $LASTEXITCODE
    
    if ($exitCode -eq 0) {
        $totalPassed++
    }
    else {
        $totalFailed++
    }
    
    return $exitCode
}

$testScripts = @(
    @{Name="Help Test"; Path="test_01_help.ps1"},
    @{Name="Case Test"; Path="test_02_case.ps1"},
    @{Name="Scenario Test"; Path="test_03_scenario.ps1"},
    @{Name="Init Test"; Path="test_04_init.ps1"}
)

foreach ($test in $testScripts) {
    $scriptPath = Join-Path $ScriptDir $test.Path
    Run-TestScript $scriptPath $test.Name
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
Write-Host " Summary" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta

if ($totalFailed -eq 0) {
    Write-Host "All tests passed! ($totalPassed/$($testScripts.Count))" -ForegroundColor Green
    exit 0
}
else {
    Write-Host "Failed tests: $totalFailed" -ForegroundColor Red
    exit 1
}
