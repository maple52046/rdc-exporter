# Domain Glossary

This is the single source of the project's ubiquitous language. The exporter is
one bounded context, so one flat list is enough. Keep code names, exported
metric names, tests, and docs aligned with the primary terms here. Full
contracts live in the Go doc comments; this table exists mainly to disambiguate
the few overloaded words.

| Term | Meaning in this project | Watch out |
| --- | --- | --- |
| Catalog | The parsed set of Entities plus the overwrite flag, merged onto the embedded default. | — |
| Definition | The framework-free domain description of one exported metric (FieldID + Name + Help + Scale), derived from the catalog. | Not the YAML `Entity`. |
| Entity | One configured metric as read from YAML (a config record, outer layer). | Config concern, not a domain type. |
| Field / FieldID | A single RDC field, identified by a numeric id (`metric.FieldID`); the unit the exporter reads from. | Not the same as the exported metric name. |
| GPU attribution | Resolving which Kubernetes workload currently uses a GPU by matching container device IDs (PCI address or render node path) to a GPU index, in order to source the workload labels. | A label source, not a metric value. |
| GPUIndex / gpu_index | Zero-based device index identifying a GPU; always the first label on every series. | The `gpu_index` label name is part of the external contract. The host discovery index (sysfs/PCI order) must line up with the RDC-reported index, or workload attribution labels the wrong GPU. |
| GPU UUID / UUID | Stable 64-bit hardware identity reported by `rocm-smi`, exported in DCGM-compatible `GPU-` plus 16 lowercase hexadecimal digits form as the `UUID` label on every series. | Discovery is best-effort at startup. An unavailable or malformed identity is exported as an empty value so label cardinality stays fixed. |
| Metric (catalog key) | In the catalog YAML, `metric` is the RDC field **enum name** (e.g. `RDC_FI_PROF_SM_ACTIVE`), used as the merge key. | Despite the word, it is not the Prometheus metric name. |
| Point | A fully prepared metric value (scaled + labelled) ready to publish. | — |
| PromName | The exported Prometheus metric name (e.g. `gpu_memory_usage`); part of the external contract. | This is the user-facing "metric name", distinct from the catalog `metric` key. |
| Sample | One raw, unscaled field reading for a GPU, before scale and labels. | — |
| Scale | Multiplier applied to a raw reading to produce the published unit; non-positive means identity (no scaling). | — |
| Workload label | The `pod`, `namespace`, and `container` labels added to a GPU's series when a Kubernetes workload is using it. | Present only when a kubelet label provider is configured; unlike the UUID identity label, these values may change during the process lifetime. |
