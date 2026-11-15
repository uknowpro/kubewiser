# Report and README design references

Reviewed: 2026-11-15. These are primary-source references for information hierarchy and explanation patterns. No external scanner is installed or invoked, and no third-party UI code, branding, claims or screenshots are copied into the product.

| Reference | Observed pattern | Kubewiser decision |
| --- | --- | --- |
| [Kubescape README](https://github.com/kubescape/kubescape) | Product purpose, feature overview, demo, quick start and command navigation | Lead both language editions with purpose, actual preview, quick start and a command index; keep detailed transcripts expandable |
| [Kubescape verbose results](https://kubescape.io/docs/scanning/#verbose-mode) | Per-resource failures with field-oriented assisted remediation and documentation | Pair every supported finding with its resource, current condition, recommendation, action and verification |
| [Polaris dashboard](https://polaris.docs.fairwinds.com/dashboard/) | Overview followed by category/namespace/workload detail | Put priority counts before individual findings; avoid an unexplained overall score |
| [Pluto quick start](https://pluto.docs.fairwinds.com/quickstart/) | API version and replacement shown together, with deprecation/removal status | Compare current API and replacement; preserve the official migration guide and behavior-change caveat |
| [Trivy misconfiguration examples](https://trivy.dev/docs/latest/guide/scanner/misconfiguration/) | Severity, problem explanation and source context | Explain why the finding matters and retain original evidence behind the readable review |
| [Popeye reports](https://github.com/derailed/popeye#report-morphology) | Resource-based findings and severity summaries in terminal/HTML | Provide labeled priority counts and readable console output; color supplements text rather than replacing it |
| [Goldilocks recommendations](https://goldilocks.docs.fairwinds.com/) | Resource recommendations derived from VPA data | Recommend measurement-based sizing without inventing CPU or memory values from static manifests |

## Report contract

- HTML, Markdown and text reviews cover scan files/snapshot/helm, upgrade/migration plans and drift. Other command results retain their structured table/JSON rendering.
- `--lang en|ko` selects review explanations, with English as the default. Original finding text, unchecked statements and source metadata are preserved and may remain English.
- JSON and MCP data schemas remain unchanged. Readable explanations are a presentation layer derived from existing findings, not additional scan evidence or an LLM judgment.
- Required, conditional and optional counts describe finding/action rows, not unique workloads or an overall health percentage.
- Current/recommended comparisons show known configuration conditions. Raw secret-like values and arbitrary manifest values are not added. Missing measurements are explicit.
- Drift is not treated as a mandate to restore the supplied manifest: defaults, controller ownership, API conversion and collection gaps require review first.
- HTML keeps original evidence and hashes in an expandable section. Markdown also includes original JSON. Text retains source/location, verification and unchecked areas.
- HTML remains standalone and script-free. Its documentation links are navigated only when clicked. All input-controlled markup is escaped.

## 한국어 요약

Kubescape의 문서 탐색 구조, Polaris의 요약→상세 흐름, Pluto의 현재 API→대체 API 비교, Kubescape/Trivy의 수정 근거 안내를 참고했습니다. 권장안은 기존 규칙의 판정에 근거하며, 실측 없이 CPU·메모리 수치를 만들거나 선언 상태를 무조건 정답으로 취급하지 않습니다. 필수·조건부·선택 조치 수는 리소스 수나 건강 점수가 아닌 발견 사항/조치 행의 개수입니다. 영어 README를 기본으로 하고 한국어판을 별도 유지합니다.
