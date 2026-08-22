param(
    [string]$GoImage = "golang:1.23"
)

$ErrorActionPreference = "Stop"
$src = (Get-Location).Path

# Unit tests run inside a Linux container against internal/wslc/wslcfake --
# no real wslc.exe involved, so this is the same command run in the
# devcontainer during day-to-day development.
Write-Host "==> unit tests (mocked wslc)"
wslc run --rm -v "${src}:/src" -w /src $GoImage sh -c "go mod tidy && go vet ./... && go test ./..."
if ($LASTEXITCODE -ne 0) {
    throw "unit tests failed"
}

# Integration tests exercise the real wslc.exe and can only run natively on
# Windows, so the test binary is cross-compiled in the same container used
# for the unit tests, then executed directly on the host.
Write-Host "==> building integration test binary"
wslc run --rm -v "${src}:/src" -w /src $GoImage sh -c "GOOS=windows GOARCH=amd64 go test -c -o integration.test.exe ./integration"
if ($LASTEXITCODE -ne 0) {
    throw "integration test build failed"
}

Write-Host "==> integration tests (native Windows, real wslc)"
& .\integration.test.exe "-test.v"
$exitCode = $LASTEXITCODE
Remove-Item .\integration.test.exe -ErrorAction SilentlyContinue
if ($exitCode -ne 0) {
    throw "integration tests failed"
}
