# Security policy

Kubewiser is pre-release software. Security fixes target the latest published prerelease; older prereleases do not receive a separate backport guarantee.

## Reporting

For a suspected vulnerability, use GitHub's **Report a vulnerability** option in the repository Security tab if private reporting is available. Do not put credentials, kubeconfigs, production snapshots, exploit secrets or private infrastructure details in public issues. If private reporting is unavailable, open an issue requesting a private reporting channel without disclosing exploit details. No response-time guarantee is currently offered.

Include the affected version/commit, operating system, minimal synthetic reproduction, expected behavior and observed impact. Use fake credentials and local fixtures.

## Boundaries

- Analysis is offline by default. There is no telemetry, automatic vulnerability submission, schema download or LLM call.
- MCP inputs and collection scope are selected by the operator, not tool arguments. Tool annotations describe behavior; code-level scope checks enforce it. See [MCP evidence](docs/mcp-trust.md).
- MCP collection requires explicit activation and uses bounded GET requests to the selected verified-TLS API origin. It rejects authentication subprocesses. CLI collection can separately authorize an authentication executable; its external effects are outside the API-origin guard.
- Read-only collection still consumes API capacity and may expose sensitive metadata to the caller. Snapshot redaction is not a universal secret detector.
- MCP clients can forward returned data to their model providers. Configure the client locally when external transmission is prohibited.
- Use trusted local manifests/charts. File limits, CEL limits and disabled Helm network functions do not make the process an operating-system sandbox.
- Reports are partial checks, not deployment authorization or proof of application compatibility.

## 보안 제보

가능하면 GitHub의 비공개 취약점 제보 기능을 사용하세요. 제공되지 않으면 공개 이슈에는 취약점 세부 내용 없이 비공개 제보 채널만 요청하세요. 실제 인증정보나 운영 스냅샷을 공개하지 마세요. 보안 수정은 최신 프리릴리스를 대상으로 하며 응답 기한이나 구버전 백포트를 보장하지 않습니다.
