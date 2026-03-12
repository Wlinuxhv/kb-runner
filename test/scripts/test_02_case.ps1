# KB Runner - Test 02: Case Management

$ProjectRoot = "D:\ai-code\kb-runnerx"
$KB_RUNNER = "$ProjectRoot\bin\kb-runner.exe"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Case Management Test" -ForegroundColor Cyan
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

Test-Cmd "list" "$KB_RUNNER list"
Test-Cmd "category filter" "$KB_RUNNER list --category security"
Test-Cmd "tag filter" "$KB_RUNNER list --tags critical"
Test-Cmd "search" "$KB_RUNNER list --search check"
Test-Cmd "show" "$KB_RUNNER show security_check"

Write-Host ""
Write-Host "Passed: $passed, Failed: $failed" -ForegroundColor $(if($failed -eq 0){"Green"}else{"Red"})
exit $failed
