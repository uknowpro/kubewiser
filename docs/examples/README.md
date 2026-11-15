# Documentation fixtures

These files are synthetic examples, not data from a production cluster.

- `manifest.yaml`: a legacy CronJob used to demonstrate an API-removal finding.
- `desired.yaml`: a LoadBalancer Service targeting port 8080.
- `snapshot.json`: one synthetic observed Service targeting port 9090, with an explicit coverage gap.
- `chart/`: a local chart rendering the same desired Service.
- `hubble.jsonl`: one synthetic flow record; it is not proof of live connectivity.
- `report.md` / `report.html` and `report.ko.md` / `report.ko.html`: actual English/Korean generated reports for `manifest.yaml`.

Run the commands in the root README from the repository root. Terminal excerpts were captured with a local development build; long JSON examples select fields from the real output. The collect example used an empty loopback mock API server and normalizes only its temporary output filename. It did not contact a production cluster.

The README screenshots are actual English/Korean browser captures of `report.html` and `report.ko.html` at a 1280 × 1100 viewport; it shows the top of the report. To regenerate it with a local Chrome installation, use a separate temporary profile:

```sh
google-chrome --headless --disable-gpu --disable-background-networking \
  --no-first-run --no-default-browser-check --hide-scrollbars \
  --force-device-scale-factor=1 --window-size=1280,1100 \
  --user-data-dir=/tmp/kubewiser-docs-chrome \
  --screenshot="$PWD/docs/images/html-report.png" \
  "file://$PWD/docs/examples/report.html"
```
