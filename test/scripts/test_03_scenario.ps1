# KB Runner - Test 03: Scenario Management

$ProjectRoot = "D:\ai-code\kb-runnerx"
$KB_RUNNER = "$ProjectRoot\bin\kb-runner.exe"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Scenario Management Test" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$passed = 0
$failed = 0

function Test-Cmd {
    param($name, $cmd)
    Write-Host -NoNewline "Test: $name ... "
    & cmd /c $cmd *> $null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "PASS" -ForegroundColor Green
        $script:passed++
    }
    else {
        Write-Host "FAIL" -ForegroundColor Red
        $script:failed++
    }
}

Test-Cmd "scenario list" "$KB_RUNNER scenario list"
Test-Cmd "scenario show" "$KB_RUNNER scenario show daily_check"

Write-Host ""
Write-Host "Passed: $passed, Failed: $failed" -ForegroundColor $(if($failed -eq 0){"Green"}else{"Red"})
exit $failed
