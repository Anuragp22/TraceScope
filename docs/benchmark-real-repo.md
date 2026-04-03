# Real-Repo Benchmark Baseline

Benchmark target:
- Repository: `gin-gonic/gin`
- Clone command: `git clone --depth 1 https://github.com/gin-gonic/gin .tmp_bench/gin`
- Host OS: Windows
- Date: 2026-04-03
- SCIP indexers available: `scip-go 0.1.26`

## Commands

```powershell
Measure-Command { go run ./cmd/tracescope index .tmp_bench\gin } | Select-Object TotalSeconds
Measure-Command { go run ./cmd/tracescope validate-scip .tmp_bench\gin } | Select-Object TotalSeconds
```

## Results

`tracescope index .tmp_bench\gin`
- Wall clock: `45.85s`
- Tool time: `43.71s`
- Graph: `1571 nodes`, `5776 edges`
- Index source: `scip`
- Indexers: `scip-go=generated`, `scip-typescript=skipped`, `scip-python=skipped`

`tracescope validate-scip .tmp_bench\gin`
- Wall clock: `3.14s`
- Nodes: `parser=1541`, `scip=1515`, `shared=1481`, `missing=60`, `extra=34`
- Edges: `parser=3996`, `scip=5667`, `shared=3460`, `missing=536`, `extra=2207`

## Interpretation

- SCIP gives a much richer graph than the parser fallback on a real Go repo, especially around `IMPLEMENTS` and method containment.
- Most remaining parser-only misses are build-tag or generated-code edge cases such as `binding_nomsgpack.go` and protobuf outputs under `testdata/`.
- This benchmark should be rerun after mapper/indexing changes to prevent performance and graph-shape regressions.
