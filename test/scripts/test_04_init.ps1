# KB Runner - Test 04: Case Init

$ProjectRoot = "D:\ai-code\kb-runnerx"
$KB_RUNNER = "$ProjectRoot\bin\kb-runner.exe"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Case Init Test" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$passed = 0
$failed = 0
$TestDir = "$ProjectRoot\test_output\test_init"

if (Test-Path $TestDir) { Remove-Item -Recurse -Force $TestDir }
New-Item -ItemType Directory -Force -Path $TestDir | Out-Null

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

function Test-File {
    param($name, $path)
    Write-Host -NoNewline "Test: $name ... "
    if (Test-Path $path) {
        Write-Host "PASS" -ForegroundColor Green
        $script:passed++
    }
    else {
        Write-Host "FAIL" -ForegroundColor Red
        $script:failed++
    }
}

Test-Cmd "init bash" "$KB_RUNNER init test_bash -o $TestDir"
Test-File "bash yaml" "$TestDir\test_bash\case.yaml"
Test-File "bash sh" "$TestDir\test_bash\run.sh"

Test-Cmd "init python" "$KB_RUNNER init test_python -l python -o $TestDir"
Test-File "python yaml" "$TestDir\test_python\case.yaml"
Test-File "python py" "$TestDir\test_python\run.py"

Write-Host ""
Write-Host "Passed: $passed, Failed: $failed" -ForegroundColor $(if($failed -eq 0){"Green"}else{"Red"})
exit $failed
