fmt: cp
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;

cp:
	cp -v ./.github/workflows/page.yml pkg/templates/page.yml
