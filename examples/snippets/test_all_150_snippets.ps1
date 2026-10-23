$snippetsDir = $PSScriptRoot
$files = Get-ChildItem -Path $snippetsDir -Filter "*.nil" | Sort-Object Name

$total = $files.Count
$passed = 0
$failed = 0

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "      Nilang Full Conformance Test Suite (1 to 150)       " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

foreach ($f in $files) {
    $out = & nil run $f.FullName 2>&1
    if ($LASTEXITCODE -eq 0 -and $out -notmatch "ত্রুটি" -and $out -notmatch "ERROR") {
        $passed++
        Write-Host ("  [PASS] {0}" -f $f.Name) -ForegroundColor Green
    } else {
        $failed++
        Write-Host ("  [FAIL] {0}" -f $f.Name) -ForegroundColor Red
        Write-Host ("         Output: {0}" -f ($out -join " ")) -ForegroundColor DarkGray
    }
}

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host ("Total Files: {0} | Passed: {1} | Failed: {2}" -f $total, $passed, $failed) -ForegroundColor Yellow
if ($failed -eq 0) {
    Write-Host "🏆 100% SUCCESS: ALL 150 NILANG SNIPPETS PASSED!" -ForegroundColor Green
} else {
    Write-Host "⚠️ SOME TESTS FAILED" -ForegroundColor Red
}
