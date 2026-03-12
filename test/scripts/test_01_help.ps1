# KB Runner - Test 01: Help Commands

$ProjectRoot = "D:\ai-code\kb-runnerx"
$KB_RUNNER = "$ProjectRoot\bin\kb-runner.exe"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Help Commands Test" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$passed = 0
$failed = 0

function Test-Cmd {
    param($name, $cmd)
    Write-Host -NoNewline "Test: $name ... "
    $result = & cmd /c $cmd 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "PASS" -ForegroundColor Green
        $script:passed++
    }
    else {
        Write-Host "FAIL" -ForegroundColor Red
        $script:failed++
    }
}

Test-Cmd "help" "$KB_RUNNER --help"
Test-Cmd "version" "$KB_RUNNER version"
Test-Cmd "list help" "$KB_RUNNER list --help"
Test-Cmd "run help" "$KB_RUNNER run --help"
Test-Cmd "init help" "$KB_RUNNER init --help"
Test-Cmd "scenario help" "$KB_RUNNER scenario --help"

Write-Host ""
Write-Host "Passed: $passed, Failed: $failed" -ForegroundColor $(if($failed -eq 0){"Green"}else{"Red"})
exit $failed
