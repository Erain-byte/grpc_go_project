$ErrorActionPreference = "Stop"

$serviceRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$projectRoot = Resolve-Path (Join-Path $serviceRoot "..")
$protoRoot = Join-Path $projectRoot "proto"
$outputRoot = Join-Path $serviceRoot "src"

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null

python -m grpc_tools.protoc `
    --proto_path=$protoRoot `
    --python_out=$outputRoot `
    --grpc_python_out=$outputRoot `
    (Join-Path $protoRoot "llm\v1\llm.proto")

if ($LASTEXITCODE -ne 0) {
    throw "failed to generate Python gRPC code"
}

Write-Host "Python gRPC code generated in $(Join-Path $outputRoot 'llm\v1')"
