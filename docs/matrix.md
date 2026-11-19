# Manifest and environment matrix / 전후 비교표

`plan upgrade` and `plan aks` always include the review matrix in JSON, HTML, Markdown and text. All listed review domains remain visible even without evidence. This is a bounded coverage inventory, not every Kubernetes feature or a compatibility certificate.

```bash
kubewiser plan upgrade --source ./before --after ./after \
  --from 1.21 --to 1.36 --cgroup-evidence ./pools.json \
  --format html --lang ko > comparison.html
kubewiser plan aks --source ./before --after ./after \
  --profile ./aks-project.json --format markdown > comparison.md
```

`--after` accepts a local YAML/JSON file or directory; for Helm, supply locally rendered manifests. AKS uses profile `cgroup_pools`. Current inputs still accept the existing source/snapshot options. Namespace defaults to `default`; specify `--namespace` consistently for manifests without namespaces.

| Status | 의미 / Meaning |
| --- | --- |
| input_missing | 입력 부족; provide evidence |
| analyzer_unsupported | 해당 일반 판정기 미지원; not automatically judged |
| execution_required | 실행 검증 필요; canary/rehearsal required |
| action_required | 검출된 필수 조치; required finding |
| review_required | 검토 필요; not a pass |
| not_detected | 지원 범위 내 미발견; not proof of absence |
| baseline_met | 제공된 목표 근거의 최소조건만 충족; not full compatibility |

The first rows summarize current-input coverage. Additional manifest-field rows show structural changes; target-validation rows independently show findings in proposed manifests. An existing finding is not automatically marked resolved simply because the API version changed. API group changes can appear as separate absent/added resources; API version changes within a group require semantic conversion review. Values are omitted, including Secret/configuration values; resource names and field paths remain visible. Built-in list normalization and supplied CRD semantics follow the existing drift engine. Missing resources in either input can reflect scope differences, not deletion. No server defaulting, admission, webhook conversion or application execution is performed.

현재 입력 검사와 변경 후 검사는 구분됩니다. 원본 필드 값은 출력하지 않고 변경 경로·추가·제거·변경 여부를 제공합니다. Secret 값도 출력하지 않지만 이름과 필드 경로는 표시됩니다. cgroup은 제공된 관측 자료/운영자 선언을 사용하며 Kubernetes 버전에서 추정하지 않습니다. 커널·런타임·드라이버의 현재 값이 없어도 목표 최소조건은 별도로 검사할 수 있습니다. cgroup v2가 아닌 대상의 상세 최소조건 검사는 미확인으로 남을 수 있습니다.

No cluster connection, node commands, uploads or external runtime lookups are added. The example HTML uses synthetic inputs, not production evidence.

## Compact grouping / 셀 병합

HTML groups rows into Manifest/API, platform/nodes, components/operations and proposed changes. Group and resource/pool cells use vertical `rowspan`; verdict cells are never merged. Rows are grouped and stably sorted by resource within each group. Markdown/text use group headings because Markdown tables do not support cell merging. JSON retains individual structured rows.

HTML은 그룹·노드풀·리소스의 중복 표기를 병합하고, 판정은 항목마다 유지합니다. 상세 근거는 펼쳐서 확인합니다.
