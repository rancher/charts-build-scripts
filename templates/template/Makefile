pull-scripts:
	./scripts/pull-scripts

{{- if (eq .Template "staging") }}
release:
	./scripts/release-assets
{{ end -}}

TARGETS := prepare patch charts clean sync validate rebase docs

$(TARGETS):
	@ls ./bin/charts-build-scripts 1>/dev/null 2>/dev/null || ./scripts/pull-scripts
	./bin/charts-build-scripts $@

.PHONY: $(TARGETS)